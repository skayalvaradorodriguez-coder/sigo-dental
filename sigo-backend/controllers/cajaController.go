package controllers

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/domain"
	"sigo-backend/facturacion"
	"sigo-backend/middleware"
	"sigo-backend/models"
	"sigo-backend/repository"
	"sigo-backend/seguridad"
	"sigo-backend/validacion"
)

// ================================================================ Caja (RF-12, RN-02)
//
// RN-02 (cierre ciego): ninguna respuesta de las rutas con caja.operar incluye el total teórico ni
// totales por medio de pago. El teórico se calcula y se guarda al cerrar, y sólo lo ve el Auditor
// (caja.revisar). La prueba TestCierreCiego revisa el JSON completo de esas respuestas.

var errCajaNoAbierta = models.NewAPIError(409, "CAJA_NO_ABIERTA", "No tiene una caja abierta. Ábrala para cobrar.")

func dinero(campo, valor string, errs validacion.Errores, permitirCero bool) domain.Centavos {
	v, err := domain.ParseDinero(valor)
	if err != nil || (!permitirCero && v == 0) {
		errs.Agregar(campo, "Monto inválido: use números con hasta 2 decimales (p. ej. 25.50).")
		return 0
	}
	if v > 99_999_999 {
		errs.Agregar(campo, "Monto demasiado alto.")
	}
	return v
}

// MiCaja — GET /api/caja/sesion: la caja abierta del recepcionista (sin montos) o {abierta:false}.
func (h *Controllers) MiCaja(c *fiber.Ctx) error {
	s, err := h.Repo.CajaAbierta(c.UserContext(), middleware.Actual(c).Usuario.ID)
	if errors.Is(err, repository.ErrNoEncontrado) {
		return c.JSON(fiber.Map{"abierta": false})
	}
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(fiber.Map{"abierta": true, "sesion": s})
}

// AbrirCaja — POST /api/caja/sesion {fondoInicial}
func (h *Controllers) AbrirCaja(c *fiber.Ctx) error {
	var in struct {
		FondoInicial string `json:"fondoInicial"`
	}
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	fondo := dinero("fondoInicial", in.FondoInicial, errs, true)
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	u := middleware.Actual(c).Usuario
	id, err := h.Repo.AbrirCaja(c.UserContext(), u.ID, fondo.String())
	if errors.Is(err, repository.ErrDuplicado) {
		return responder(c, models.NewAPIError(409, "CAJA_YA_ABIERTA", "Ya tiene una caja abierta."))
	}
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "CAJA_APERTURA", Entidad: "caja_sesion", EntidadID: id, Detalle: map[string]any{"fondoInicial": fondo.String()}})
	s, err := h.Repo.CajaAbierta(c.UserContext(), u.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.Status(201).JSON(fiber.Map{"abierta": true, "sesion": s})
}

// CerrarCaja — POST /api/caja/sesion/cerrar {efectivo, tarjeta, transferencia, nota?}
// Cierre ciego: responde sólo la confirmación, nunca el teórico ni la diferencia.
func (h *Controllers) CerrarCaja(c *fiber.Ctx) error {
	var in struct {
		Efectivo      string `json:"efectivo"`
		Tarjeta       string `json:"tarjeta"`
		Transferencia string `json:"transferencia"`
		Nota          string `json:"nota"`
	}
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	dec := map[string]string{
		domain.MedioEfectivo:      dinero("efectivo", in.Efectivo, errs, true).String(),
		domain.MedioTarjeta:       dinero("tarjeta", in.Tarjeta, errs, true).String(),
		domain.MedioTransferencia: dinero("transferencia", in.Transferencia, errs, true).String(),
	}
	errs.Longitud("nota", strings.TrimSpace(in.Nota), 0, 1000, "Nota: hasta 1000 caracteres.")
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	ctx := c.UserContext()
	s, err := h.Repo.CajaAbierta(ctx, middleware.Actual(c).Usuario.ID)
	if errors.Is(err, repository.ErrNoEncontrado) {
		return responder(c, errCajaNoAbierta)
	}
	if err != nil {
		return h.errorInterno(c, err)
	}
	if err := h.Repo.CerrarCaja(ctx, s.ID, dec, validacion.Opcional(strings.TrimSpace(in.Nota))); err != nil {
		if errors.Is(err, repository.ErrConflicto) {
			return responder(c, errCajaNoAbierta)
		}
		return h.errorInterno(c, err)
	}
	// La auditoría guarda sólo lo declarado (lo que el recepcionista ya sabe).
	h.Auditar(c, repository.Evento{Accion: "CAJA_CIERRE", Entidad: "caja_sesion", EntidadID: s.ID, Detalle: map[string]any{"declarado": dec}})
	return c.JSON(fiber.Map{"cerrada": true, "sesionId": s.ID, "cobros": s.Cobros,
		"mensaje": "Cierre registrado. La conciliación la revisa el Auditor."})
}

// MisCobros — GET /api/caja/cobros: cobros de la caja abierta (cada uno con su total; sin sumas).
func (h *Controllers) MisCobros(c *fiber.Ctx) error {
	ctx := c.UserContext()
	s, err := h.Repo.CajaAbierta(ctx, middleware.Actual(c).Usuario.ID)
	if errors.Is(err, repository.ErrNoEncontrado) {
		return c.JSON([]models.Cobro{})
	}
	if err != nil {
		return h.errorInterno(c, err)
	}
	cs, err := h.Repo.CobrosSesion(ctx, s.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(cs)
}

// PorCobrar — GET /api/pacientes/:id/por-cobrar
func (h *Controllers) PorCobrar(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Paciente no encontrado.")
	}
	xs, err := h.Repo.PorCobrar(c.UserContext(), id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(xs)
}

// CobrosPaciente — GET /api/pacientes/:id/cobros
func (h *Controllers) CobrosPaciente(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Paciente no encontrado.")
	}
	cs, err := h.Repo.CobrosPaciente(c.UserContext(), id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "cobros", EntidadID: id})
	return c.JSON(cs)
}

type cobroIn struct {
	PacienteID string `json:"pacienteId"`
	Conceptos  []struct {
		ProcedimientoRealizadoID string `json:"procedimientoRealizadoId"`
		ProcedimientoID          string `json:"procedimientoId"`
	} `json:"conceptos"`
	Pagos []struct {
		Medio      string `json:"medio"`
		Monto      string `json:"monto"`
		Referencia string `json:"referencia"`
		ConvenioID string `json:"convenioId"`
	} `json:"pagos"`
}

// CrearCobro — POST /api/cobros. Los montos de los conceptos los fija el sistema (presupuesto
// aprobado o arancel); el recepcionista sólo reparte el total entre los medios de pago.
func (h *Controllers) CrearCobro(c *fiber.Ctx) error {
	var in cobroIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	errs.UUID("pacienteId", in.PacienteID, true)
	if len(in.Conceptos) == 0 || len(in.Conceptos) > 30 {
		errs.Agregar("conceptos", "Seleccione entre 1 y 30 conceptos a cobrar.")
	}
	if len(in.Pagos) == 0 || len(in.Pagos) > 10 {
		errs.Agregar("pagos", "Registre entre 1 y 10 pagos.")
	}
	pagos := []domain.PagoMedio{}
	for i, p := range in.Pagos {
		k := fmt.Sprintf("pagos[%d]", i)
		errs.EnLista(k+".medio", p.Medio, domain.MediosPago, "Medio: EFECTIVO, TARJETA, TRANSFERENCIA o SEGURO.")
		m := dinero(k+".monto", p.Monto, errs, false)
		pagos = append(pagos, domain.PagoMedio{Medio: p.Medio, Monto: m})
		ref := strings.TrimSpace(p.Referencia)
		if (p.Medio == domain.MedioTarjeta || p.Medio == domain.MedioTransferencia) && ref == "" {
			errs.Agregar(k+".referencia", "Indique el número de comprobante del pago (voucher o transferencia).")
		}
		errs.Longitud(k+".referencia", ref, 0, 60, "Referencia: hasta 60 caracteres.")
		if p.Medio == domain.MedioSeguro {
			errs.UUID(k+".convenioId", p.ConvenioID, true)
		} else if p.ConvenioID != "" {
			errs.Agregar(k+".convenioId", "Sólo el pago con seguro lleva convenio.")
		}
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	ctx := c.UserContext()
	u := middleware.Actual(c).Usuario
	s, err := h.Repo.CajaAbierta(ctx, u.ID)
	if errors.Is(err, repository.ErrNoEncontrado) {
		return responder(c, errCajaNoAbierta)
	}
	if err != nil {
		return h.errorInterno(c, err)
	}
	p, err := h.Repo.PacientePorID(ctx, in.PacienteID, h.Hoy())
	if err != nil {
		return h.errorRepo(c, err, "Paciente no encontrado.", "")
	}
	pend, err := h.Repo.PorCobrar(ctx, p.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	procs, err := h.Repo.Procedimientos(ctx, true)
	if err != nil {
		return h.errorInterno(c, err)
	}
	conceptos := []models.CobroConcepto{}
	var total domain.Centavos
	vistos := map[string]bool{}
	for i, x := range in.Conceptos {
		k := fmt.Sprintf("conceptos[%d]", i)
		switch {
		case x.ProcedimientoRealizadoID != "":
			j := slices.IndexFunc(pend, func(y models.PorCobrar) bool { return y.ProcedimientoRealizadoID == x.ProcedimientoRealizadoID })
			if j < 0 || vistos[x.ProcedimientoRealizadoID] {
				errs.Agregar(k, "Ese procedimiento no está pendiente de cobro para este paciente.")
				continue
			}
			vistos[x.ProcedimientoRealizadoID] = true
			y := pend[j]
			desc := y.Codigo + " · " + y.Procedimiento
			if y.Diente != nil {
				desc += fmt.Sprintf(" (diente %d)", *y.Diente)
			}
			rid := y.ProcedimientoRealizadoID
			conceptos = append(conceptos, models.CobroConcepto{ProcedimientoRealizadoID: &rid, ProcedimientoID: y.ProcedimientoID, Descripcion: desc, Monto: y.Monto})
			m, _ := domain.ParseDinero(y.Monto)
			total += m
		case validacion.EsUUID(x.ProcedimientoID):
			j := slices.IndexFunc(procs, func(y models.Procedimiento) bool { return y.ID == x.ProcedimientoID })
			m := domain.Centavos(0)
			if j >= 0 {
				m, _ = domain.ParseDinero(procs[j].Arancel)
			}
			if j < 0 || m <= 0 {
				errs.Agregar(k, "Procedimiento inexistente, inactivo o sin arancel.")
				continue
			}
			conceptos = append(conceptos, models.CobroConcepto{ProcedimientoID: procs[j].ID, Descripcion: procs[j].Codigo + " · " + procs[j].Nombre, Monto: m.String()})
			total += m
		default:
			errs.Agregar(k, "Indique el procedimiento realizado o el procedimiento del catálogo.")
		}
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	if err := domain.ValidarPagoMixto(total, pagos); err != nil {
		return responder(c, models.NewAPIError(400, "PAGO_NO_CUADRA", "El pago no cuadra: "+err.Error()+".").Con("total", total.String()))
	}
	convs, err := h.Repo.Convenios(ctx, true)
	if err != nil {
		return h.errorInterno(c, err)
	}
	lineas := []models.CobroPago{}
	for i, x := range in.Pagos {
		lp := models.CobroPago{Medio: x.Medio, Monto: pagos[i].Monto.String(), Referencia: validacion.Opcional(strings.TrimSpace(x.Referencia))}
		if x.Medio == domain.MedioSeguro {
			if !slices.ContainsFunc(convs, func(cv models.Convenio) bool { return cv.ID == x.ConvenioID }) {
				return malaPeticion(c, "CONVENIO_INVALIDO", "El convenio no existe o está inactivo.")
			}
			cid := x.ConvenioID
			lp.ConvenioID = &cid
		}
		lineas = append(lineas, lp)
	}
	id, err := h.Repo.CrearCobro(ctx, repository.NuevoCobro{SesionID: s.ID, PacienteID: p.ID, UsuarioID: u.ID, Total: total.String(), Conceptos: conceptos, Pagos: lineas})
	switch {
	case errors.Is(err, repository.ErrCajaCerrada):
		return responder(c, errCajaNoAbierta)
	case errors.Is(err, repository.ErrDuplicado):
		return responder(c, models.NewAPIError(409, "YA_COBRADO", "Uno de los procedimientos ya fue cobrado. Recargue la lista."))
	case err != nil:
		return h.errorRepo(c, err, "", "")
	}
	h.Auditar(c, repository.Evento{Accion: "COBRO", Entidad: "cobro", EntidadID: id, Detalle: map[string]any{"paciente": p.ID, "conceptos": len(conceptos), "pagos": len(lineas)}})
	cb, err := h.Repo.CobroPorID(ctx, id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	// El comprobante se emite después de guardar el cobro: si el servicio no responde, el cobro
	// queda registrado y la factura se reintenta (POST /cobros/:id/comprobante).
	h.emitir(c, cb, p, "FACTURA", "")
	return h.responderCobro(c, 201, id)
}

func (h *Controllers) responderCobro(c *fiber.Ctx, status int, id string) error {
	cb, err := h.Repo.CobroPorID(c.UserContext(), id)
	if err != nil {
		return h.errorRepo(c, err, "Cobro no encontrado.", "")
	}
	return c.Status(status).JSON(cb)
}

// ObtenerCobro — GET /api/cobros/:id
func (h *Controllers) ObtenerCobro(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Cobro no encontrado.")
	}
	return h.responderCobro(c, 200, id)
}

func vigente(cb *models.Cobro, tipo string) *models.Comprobante {
	for i := len(cb.Comprobantes) - 1; i >= 0; i-- {
		if cb.Comprobantes[i].Tipo == tipo {
			return &cb.Comprobantes[i]
		}
	}
	return nil
}

// emitir arma el documento, lo envía al proveedor y registra el intento (autorizado o no).
func (h *Controllers) emitir(c *fiber.Ctx, cb *models.Cobro, p *models.Paciente, tipo, motivo string) *models.Comprobante {
	ctx := c.UserContext()
	nombre, doc, email := p.Nombres+" "+p.Apellidos, documentoLegible(p.TipoDocumento, p.NumeroDocumento), p.Email
	if p.EsMenor && p.Tutor != nil { // el comprobante va a nombre de quien responde por el menor
		nombre, doc, email = p.Tutor.Nombre, documentoLegible(p.Tutor.TipoDocumento, p.Tutor.NumeroDocumento), p.Tutor.Email
	}
	d := facturacion.Documento{Tipo: tipo, Emisor: h.Cfg.NombreClinica, Receptor: facturacion.Receptor{Documento: doc, Nombre: nombre},
		Total: cb.Total, Fecha: time.Now().UTC().Format(time.RFC3339), ReciboInterno: cb.Numero, Motivo: motivo}
	if email != nil {
		d.Receptor.Email = *email
	}
	for _, x := range cb.Conceptos {
		d.Lineas = append(d.Lineas, facturacion.Linea{Descripcion: x.Descripcion, Monto: x.Monto})
	}
	for _, x := range cb.Pagos {
		d.Pagos = append(d.Pagos, facturacion.Pago{Medio: x.Medio, Monto: x.Monto})
	}
	if tipo == "NOTA_CREDITO" {
		if f := vigente(cb, "FACTURA"); f != nil && f.Numero != nil {
			d.Referencia = *f.Numero
		}
	}
	sec, err := h.Repo.SiguienteSecuencial(ctx, tipo)
	if err != nil {
		_ = h.errorInterno(c, err)
		return nil
	}
	d.Secuencial = sec
	n := repository.NuevoComprobante{CobroID: cb.ID, Tipo: tipo, Proveedor: h.Facturacion.Nombre(), Simulado: h.Facturacion.Simulado(),
		UsuarioID: middleware.Actual(c).Usuario.ID, Secuencial: sec, Documento: d, Hash: seguridad.SHA256Hex(repository.JSONCanonico(d))}
	res, err := h.Facturacion.Emitir(ctx, d)
	if err != nil {
		msg := "El servicio de facturación no respondió: " + err.Error()
		n.Estado, n.Mensaje = "ERROR", &msg
	} else {
		n.Estado = res.Estado
		n.Mensaje = validacion.Opcional(res.Mensaje)
		n.Numero, n.Autorizacion = validacion.Opcional(res.Numero), validacion.Opcional(res.Autorizacion)
	}
	if err := h.Repo.RegistrarComprobante(ctx, n); err != nil {
		if errors.Is(err, repository.ErrDuplicado) { // otro intento simultáneo ya quedó autorizado
			return &models.Comprobante{Tipo: tipo, Estado: "AUTORIZADO"}
		}
		_ = h.errorInterno(c, err)
		return nil
	}
	h.Auditar(c, repository.Evento{Accion: "COMPROBANTE", Entidad: "cobro", EntidadID: cb.ID,
		Detalle: map[string]any{"tipo": tipo, "estado": n.Estado, "numero": n.Numero, "simulado": n.Simulado}})
	return &models.Comprobante{Tipo: tipo, Estado: n.Estado, Numero: n.Numero}
}

// ReintentarComprobante — POST /api/cobros/:id/comprobante: emite la factura (o la nota de crédito
// de un cobro anulado) si el último intento no quedó autorizado.
func (h *Controllers) ReintentarComprobante(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Cobro no encontrado.")
	}
	ctx := c.UserContext()
	cb, err := h.Repo.CobroPorID(ctx, id)
	if err != nil {
		return h.errorRepo(c, err, "Cobro no encontrado.", "")
	}
	tipo, motivo := "FACTURA", ""
	if f := vigente(cb, "FACTURA"); f != nil && f.Estado == "AUTORIZADO" {
		if cb.Anulacion == nil {
			return responder(c, models.NewAPIError(409, "YA_AUTORIZADO", "La factura ya está autorizada."))
		}
		if nc := vigente(cb, "NOTA_CREDITO"); nc != nil && nc.Estado == "AUTORIZADO" {
			return responder(c, models.NewAPIError(409, "YA_AUTORIZADO", "La nota de crédito ya está autorizada."))
		}
		tipo, motivo = "NOTA_CREDITO", cb.Anulacion.Motivo
	} else if cb.Anulacion != nil {
		return responder(c, models.NewAPIError(409, "COBRO_ANULADO", "El cobro está anulado y nunca tuvo factura autorizada: no hay nada que emitir."))
	}
	p, err := h.Repo.PacientePorID(ctx, cb.PacienteID, h.Hoy())
	if err != nil {
		return h.errorInterno(c, err)
	}
	if h.emitir(c, cb, p, tipo, motivo) == nil {
		return nil // ya respondió con el error
	}
	return h.responderCobro(c, 200, id)
}

// AnularCobro — POST /api/cobros/:id/anular {motivo}: sólo en la caja abierta del mismo
// recepcionista. Si tenía factura autorizada, se emite la nota de crédito.
func (h *Controllers) AnularCobro(c *fiber.Ctx) error {
	var in struct {
		Motivo string `json:"motivo"`
	}
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Cobro no encontrado.")
	}
	in.Motivo = strings.TrimSpace(in.Motivo)
	errs := validacion.Errores{}
	errs.Longitud("motivo", in.Motivo, 5, 500, "Explique el motivo (5 a 500 caracteres).")
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	ctx := c.UserContext()
	u := middleware.Actual(c).Usuario
	estado, cajero, err := h.Repo.EstadoCajaDeCobro(ctx, id)
	if err != nil {
		return h.errorRepo(c, err, "Cobro no encontrado.", "")
	}
	if estado != "ABIERTA" || cajero != u.ID {
		return responder(c, models.NewAPIError(409, "ANULACION_NO_PERMITIDA", "Sólo se anula un cobro de su propia caja mientras sigue abierta."))
	}
	err = h.Repo.AnularCobro(ctx, id, in.Motivo, u.ID)
	switch {
	case errors.Is(err, repository.ErrDuplicado):
		return responder(c, models.NewAPIError(409, "YA_ANULADO", "El cobro ya está anulado."))
	case errors.Is(err, repository.ErrCajaCerrada):
		return responder(c, models.NewAPIError(409, "ANULACION_NO_PERMITIDA", "La caja ya se cerró."))
	case err != nil:
		return h.errorRepo(c, err, "Cobro no encontrado.", "")
	}
	h.Auditar(c, repository.Evento{Accion: "ANULACION", Entidad: "cobro", EntidadID: id, Detalle: map[string]any{"motivo": in.Motivo}})
	cb, err := h.Repo.CobroPorID(ctx, id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	if f := vigente(cb, "FACTURA"); f != nil && f.Estado == "AUTORIZADO" {
		if p, err := h.Repo.PacientePorID(ctx, cb.PacienteID, h.Hoy()); err == nil {
			h.emitir(c, cb, p, "NOTA_CREDITO", in.Motivo)
		}
	}
	return h.responderCobro(c, 200, id)
}

// ================================================================ Auditor (caja.revisar)

func (h *Controllers) conciliar(c *fiber.Ctx, x *models.CierreCaja) {
	if len(x.Teorico) == 0 {
		x.Diferencias = []domain.Diferencia{}
		return
	}
	teo, dec := map[string]domain.Centavos{}, map[string]domain.Centavos{}
	for _, m := range domain.MediosEnCaja {
		teo[m], _ = domain.ParseDinero(x.Teorico[m])
		dec[m], _ = domain.ParseDinero(x.Declarado[m])
	}
	d, ok := domain.ConciliarCierre(teo, dec, int64(h.Ajuste(c.UserContext(), repository.AjusteToleranciaCaja)))
	x.Diferencias, x.Cuadra = d, &ok
}

// ListarCierres — GET /api/caja/cierres?estado=ABIERTA|CERRADA|REVISADA
func (h *Controllers) ListarCierres(c *fiber.Ctx) error {
	estado := c.Query("estado")
	if estado != "" && !slices.Contains([]string{"ABIERTA", "CERRADA", "REVISADA"}, estado) {
		return malaPeticion(c, "VALIDACION", "Estado inválido.")
	}
	xs, err := h.Repo.Cierres(c.UserContext(), estado)
	if err != nil {
		return h.errorInterno(c, err)
	}
	for i := range xs {
		h.conciliar(c, &xs[i])
	}
	return c.JSON(xs)
}

// ObtenerCierre — GET /api/caja/cierres/:id (con los cobros de la sesión)
func (h *Controllers) ObtenerCierre(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Cierre no encontrado.")
	}
	ctx := c.UserContext()
	x, err := h.Repo.CierrePorID(ctx, id)
	if err != nil {
		return h.errorRepo(c, err, "Cierre no encontrado.", "")
	}
	h.conciliar(c, x)
	cs, err := h.Repo.CobrosSesion(ctx, id)
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "LECTURA", Entidad: "caja_sesion", EntidadID: id})
	return c.JSON(fiber.Map{"cierre": x, "cobros": cs})
}

// RevisarCierre — POST /api/caja/cierres/:id/revisar {nota}: un descuadre exige nota.
func (h *Controllers) RevisarCierre(c *fiber.Ctx) error {
	var in struct {
		Nota string `json:"nota"`
	}
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return noEncontrado(c, "Cierre no encontrado.")
	}
	ctx := c.UserContext()
	x, err := h.Repo.CierrePorID(ctx, id)
	if err != nil {
		return h.errorRepo(c, err, "Cierre no encontrado.", "")
	}
	if x.Estado != "CERRADA" {
		return responder(c, models.NewAPIError(409, "NO_REVISABLE", "Sólo se revisa una caja cerrada y aún sin revisar."))
	}
	h.conciliar(c, x)
	nota := strings.TrimSpace(in.Nota)
	if !*x.Cuadra && len([]rune(nota)) < 5 {
		return errValidacion(c, validacion.Errores{"nota": "El cierre tiene diferencias: explique cómo se resolvieron (al menos 5 caracteres)."})
	}
	if len([]rune(nota)) > 1000 {
		return errValidacion(c, validacion.Errores{"nota": "Nota: hasta 1000 caracteres."})
	}
	if err := h.Repo.RevisarCierre(ctx, id, middleware.Actual(c).Usuario.ID, validacion.Opcional(nota)); err != nil {
		return h.errorRepo(c, err, "Cierre no encontrado.", "")
	}
	h.Auditar(c, repository.Evento{Accion: "CAJA_REVISION", Entidad: "caja_sesion", EntidadID: id, Detalle: map[string]any{"cuadra": *x.Cuadra, "diferencias": x.Diferencias}})
	return h.ObtenerCierre(c)
}

// CuentasPorCobrar — GET /api/caja/cuentas-por-cobrar: lo pagado con seguro, por convenio (S-06).
func (h *Controllers) CuentasPorCobrar(c *fiber.Ctx) error {
	xs, err := h.Repo.CuentasPorCobrarSeguros(c.UserContext())
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(xs)
}

// EstadoFacturacion — GET /api/facturacion/proveedor: indica si el proveedor es simulado.
func (h *Controllers) EstadoFacturacion(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"nombre": h.Facturacion.Nombre(), "simulado": h.Facturacion.Simulado()})
}
