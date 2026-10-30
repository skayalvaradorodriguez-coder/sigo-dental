package routes

import (
	"encoding/json"
	"sort"
	"strings"
)

// Doc describe una ruta para el OpenAPI. La política de acceso NO se escribe aquí: se toma del
// registro real de rutas (Registro.Politica), así la documentación no puede contradecir al código.
type Doc struct {
	Resumen string
	Cuerpo  string // campos del JSON de entrada (si aplica)
	Exito   int    // código HTTP de éxito
}

// Docs: una entrada por ruta. pruebas/auth_test.go falla si se registra una ruta sin documentar.
var Docs = map[string]Doc{
	"GET /api/health":                                 {"Estado de la API, la base y Redis (ok | degraded).", "", 200},
	"POST /api/login":                                 {"Primer factor. Devuelve {token, siguientePaso}. Límite: 10/min por IP.", "{email, password}", 200},
	"POST /api/mfa/verificar":                         {"Segundo factor (TOTP RFC 6238, anti-replay). Rota el token.", "{codigo}", 200},
	"POST /api/mfa/configurar":                        {"Genera el secreto TOTP y la URL otpauth:// para el QR.", "", 200},
	"POST /api/mfa/activar":                           {"Activa MFA con el primer código válido. Rota el token.", "{codigo}", 200},
	"POST /api/password":                              {"Cambio de contraseña (obligatorio en el primer ingreso).", "{actual, nueva}", 204},
	"POST /api/logout":                                {"Revoca la sesión actual.", "", 204},
	"GET /api/me":                                     {"Perfil, permisos, sesión (minutosInactividad) y siguientePaso.", "", 200},
	"POST /api/heartbeat":                             {"Marca actividad del usuario (evita el cierre por inactividad).", "", 204},
	"GET /api/panel":                                  {"Widgets del panel según los permisos del rol.", "", 200},
	"GET /api/usuarios":                               {"Lista de usuarios (sin hashes ni secretos MFA).", "", 200},
	"GET /api/usuarios/roles":                         {"Matriz rol → permisos.", "", 200},
	"POST /api/usuarios":                              {"Alta de usuario; devuelve una contraseña temporal (se muestra una vez).", "{email, nombre, rol, registroProfesional?}", 201},
	"PATCH /api/usuarios/:id":                         {"Cambia nombre, rol, registro o activo (desactivar revoca sesiones).", "{nombre?, rol?, registroProfesional?, activo?}", 200},
	"POST /api/usuarios/:id/restablecer-password":     {"Genera contraseña temporal y revoca sesiones.", "", 200},
	"POST /api/usuarios/:id/restablecer-mfa":          {"Elimina el MFA del usuario y revoca sesiones.", "", 204},
	"GET /api/ajustes":                                {"Parámetros configurables con rango y valor por defecto.", "", 200},
	"PUT /api/ajustes/:clave":                         {"Guarda un parámetro (auditado con el valor anterior).", "{valor}", 200},
	"GET /api/auditoria":                              {"Bitácora paginada. Filtros: accion, entidad, entidadId, usuarioId, desde, hasta, page, limit.", "", 200},
	"POST /api/auditoria/verificar":                   {"Recorre la cadena de hashes SHA-256 y reporta si hay alteraciones.", "", 200},
	"GET /api/catalogos/estructura":                   {"Sedes → consultorios → sillones (?todos=true incluye inactivos, sólo admin). Caché Redis.", "", 200},
	"GET /api/catalogos/sillones":                     {"Sillones activos con consultorio y sede. Caché Redis (X-Cache).", "", 200},
	"GET /api/catalogos/procedimientos":               {"Procedimientos (código, duración, arancel). Caché Redis.", "", 200},
	"GET /api/catalogos/convenios":                    {"Seguros y convenios. Caché Redis.", "", 200},
	"GET /api/catalogos/odontologos":                  {"Odontólogos activos (para la agenda).", "", 200},
	"POST /api/catalogos/:tipo":                       {"Crea en sedes | consultorios | sillones | procedimientos | convenios. Invalida la caché.", "según :tipo", 201},
	"PATCH /api/catalogos/:tipo/:id":                  {"Modifica o desactiva (nunca se borra). Invalida la caché.", "según :tipo", 200},
	"GET /api/pacientes":                              {"Búsqueda paginada (q, page, limit) con alertas. La búsqueda se audita.", "", 200},
	"GET /api/pacientes/:id":                          {"Ficha única con tutor y alertas (lectura auditada).", "", 200},
	"POST /api/pacientes":                             {"Alta (RF-01). Menor de edad ⇒ tutor obligatorio. Documento único.", "{tipoDocumento, numeroDocumento, nombres, apellidos, fechaNacimiento, sexo, telefono?, email?, direccion?, emergenciaNombre, emergenciaTelefono, emergenciaParentesco?, canalPreferido, autorizaMensajes, convenioId?, numeroAfiliado?, tutor?}", 201},
	"PUT /api/pacientes/:id":                          {"Reemplazo completo de la ficha; auditoría con valor anterior y nuevo. quitarTutor=true retira el tutor (sólo adultos).", "igual que POST + {quitarTutor?}", 200},
	"GET /api/pacientes/:id/anamnesis":                {"Historial de versiones de la anamnesis (lectura auditada).", "", 200},
	"POST /api/pacientes/:id/anamnesis":               {"Nueva versión de anamnesis (RF-02); las anteriores son inmutables.", "{enfermedadesSistemicas[], alergias[{sustancia,tipo,reaccion?}], coagulopatia, anticoagulado, embarazo?, antecedentesQuirurgicos?, medicacionActual?, otraCondicionCritica?, observaciones?}", 201},
	"GET /api/citas":                                  {"Citas en un rango (desde, hasta ISO-8601; máx. 31 días). Filtros: sillonId, odontologoId, pacienteId.", "", 200},
	"GET /api/citas/:id":                              {"Detalle con historial de estados y mensajes programados.", "", 200},
	"POST /api/citas":                                 {"Agenda una cita (RF-09). Sillón ocupado ⇒ 409 SILLON_OCUPADO con extra.conflicto (RN-03).", "{pacienteId, odontologoId, sillonId, procedimientoId?, inicio, duracionMin, motivo?}", 201},
	"PATCH /api/citas/:id/reprogramar":                {"Nuevo horario/sillón/odontólogo; rehace los recordatorios pendientes.", "{inicio, duracionMin, sillonId?, odontologoId?}", 200},
	"POST /api/citas/:id/estado":                      {"Cambio de estado (RF-10). CANCELADO exige nota; NO_ASISTIO sólo desde la hora de la cita.", "{estado, nota?}", 200},
	"GET /api/notificaciones":                         {"Bandeja de mensajes (RF-11) e indicador de proveedor simulado.", "", 200},
	"POST /api/notificaciones/procesar":               {"Procesa ahora los mensajes vencidos (también corre cada 30 s en segundo plano).", "", 200},
	"GET /api/pacientes/:id/odontograma":              {"Odontograma (RF-03): catálogo de hallazgos, estado vigente e historial completo (lectura auditada). Dientes en FDI.", "", 200},
	"POST /api/pacientes/:id/odontograma":             {"Agrega o quita un hallazgo (evento; nunca se borra). Anamnesis pendiente ⇒ 409 ANAMNESIS_PENDIENTE.", "{diente (FDI), superficie? (M|D|O|I|V|L|P), hallazgo, accion (AGREGAR|QUITAR), nota?}", 201},
	"GET /api/pacientes/:id/periodontogramas":         {"Periodontogramas (RF-04) con NIC calculado por la BD y resumen (sangrado, bolsas ≥ 4 mm, NIC máximo).", "", 200},
	"POST /api/pacientes/:id/periodontogramas":        {"Examen completo: 6 sitios por diente presente; no se edita después.", "{observaciones?, dientes[{diente, ausente, movilidad?, furca?, sitios[{sitio, profundidad, margen, sangrado, supuracion}]}]}", 201},
	"GET /api/pacientes/:id/notas":                    {"Notas SOEP (RF-05) con estado (BORRADOR, FIRMADA, BLOQUEADA_SIN_FIRMA), integridad del hash y adendas.", "", 200},
	"POST /api/pacientes/:id/notas":                   {"Nueva nota; bloqueo_en = ahora + clinica.horasBloqueoNotas (24 h por defecto).", "{citaId?, subjetivo, objetivo, evaluacion, plan}", 201},
	"PUT /api/notas/:id":                              {"Edita el borrador: sólo el autor, sin firma y antes del bloqueo (409 NOTA_FIRMADA / NOTA_BLOQUEADA). La BD también lo impide.", "{subjetivo, objetivo, evaluacion, plan}", 200},
	"POST /api/notas/:id/firmar":                      {"Firma electrónica simple: re-autenticación con código MFA + hash SHA-256 del contenido. No es firma digital con certificado.", "{codigo}", 200},
	"POST /api/notas/:id/adendas":                     {"Adenda firmada a una nota firmada o bloqueada (la nota original no cambia).", "{texto, codigo}", 201},
	"GET /api/pacientes/:id/procedimientos":           {"Procedimientos realizados con sus adendas.", "", 200},
	"POST /api/pacientes/:id/procedimientos":          {"Registra un procedimiento realizado, firmado e inmutable (RN-01: sin edición ni borrado, también en la BD).", "{procedimientoId, citaId?, diente?, superficies?[], observaciones?, codigo}", 201},
	"POST /api/procedimientos-realizados/:id/adendas": {"Corrección de un procedimiento realizado mediante adenda firmada (RN-01).", "{texto, codigo}", 201},
}

// OpenAPI arma un documento OpenAPI 3.0 a partir del registro de rutas.
func OpenAPI(registro []Registro) ([]byte, error) {
	rutas := map[string]map[string]any{}
	sort.Slice(registro, func(i, j int) bool { return registro[i].Ruta < registro[j].Ruta })
	for _, r := range registro {
		d := Docs[r.Metodo+" "+r.Ruta]
		ruta := r.Ruta
		var params []any
		for _, seg := range strings.Split(ruta, "/") {
			if strings.HasPrefix(seg, ":") {
				params = append(params, map[string]any{"name": seg[1:], "in": "path", "required": true, "schema": map[string]string{"type": "string"}})
				ruta = strings.Replace(ruta, seg, "{"+seg[1:]+"}", 1)
			}
		}
		etiqueta := strings.Split(strings.TrimPrefix(r.Ruta, "/api/"), "/")[0]
		op := map[string]any{
			"summary":     d.Resumen,
			"tags":        []string{etiqueta},
			"x-politica":  r.Politica,
			"description": "Acceso: " + describir(r.Politica) + cuerpo(d.Cuerpo),
			"responses":   respuestas(d.Exito, r.Politica),
		}
		if len(params) > 0 {
			op["parameters"] = params
		}
		if r.Politica != "publica" {
			op["security"] = []map[string][]string{{"bearer": {}}}
		}
		if d.Cuerpo != "" {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": map[string]string{"type": "object"}}}}
		}
		if rutas[ruta] == nil {
			rutas[ruta] = map[string]any{}
		}
		rutas[ruta][strings.ToLower(r.Metodo)] = op
	}
	doc := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "SIGO-Dental API",
			"version":     "0.2.0",
			"description": "Generado con `go run . openapi` desde el registro real de rutas (routes.go). Errores: {status, code, message, error, extra?}. Todos los datos de ejemplo son ficticios.",
		},
		"servers": []map[string]string{{"url": "http://localhost:3000"}},
		"components": map[string]any{
			"securitySchemes": map[string]any{"bearer": map[string]string{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"}},
			"schemas": map[string]any{"APIError": map[string]any{"type": "object", "properties": map[string]any{
				"status": map[string]string{"type": "integer"}, "code": map[string]string{"type": "string"},
				"message": map[string]string{"type": "string"}, "error": map[string]string{"type": "string"},
				"extra": map[string]string{"type": "object"}}}},
		},
		"paths": rutas,
	}
	return json.MarshalIndent(doc, "", "  ")
}

func describir(p string) string {
	switch {
	case p == "publica":
		return "pública."
	case p == "sesion":
		return "token válido (aunque falte MFA o cambio de contraseña)."
	case p == "completa":
		return "sesión completa (MFA verificado)."
	case strings.HasPrefix(p, "permisos:"):
		return "sesión completa + permiso " + strings.ReplaceAll(strings.TrimPrefix(p, "permisos:"), "+", " y ") + "."
	default:
		return "sesión completa + alguno de: " + strings.ReplaceAll(strings.TrimPrefix(p, "alguno:"), "|", ", ") + "."
	}
}

func cuerpo(c string) string {
	if c == "" {
		return ""
	}
	return "\n\nCuerpo JSON: `" + c + "` (campos desconocidos ⇒ 400)."
}

func respuestas(exito int, politica string) map[string]any {
	err := map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": map[string]string{"$ref": "#/components/schemas/APIError"}}}}
	con := func(desc string) map[string]any {
		r := map[string]any{"description": desc}
		for k, v := range err {
			r[k] = v
		}
		return r
	}
	if exito == 0 {
		exito = 200
	}
	r := map[string]any{itoa3(exito): map[string]string{"description": "Éxito"}, "400": con("VALIDACION u otro error de entrada")}
	if politica != "publica" {
		r["401"] = con("NO_AUTENTICADO / SESION_EXPIRADA")
		r["403"] = con("PROHIBIDO / MFA_REQUERIDO / CAMBIO_PASSWORD_REQUERIDO")
	}
	return r
}

func itoa3(n int) string {
	return string(rune('0'+n/100)) + string(rune('0'+n/10%10)) + string(rune('0'+n%10))
}
