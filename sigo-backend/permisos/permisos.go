// Package permisos: catálogo de permisos y matriz rol → permisos (RBAC, RNF-05).
// Cada ruta declara qué permiso exige y el middleware lo verifica en el backend
// (la interfaz sólo oculta opciones; no protege).
package permisos

import "slices"

const (
	// Administración
	UsuariosGestionar  = "usuarios.gestionar"
	AjustesGestionar   = "ajustes.gestionar"
	CatalogosGestionar = "catalogos.gestionar"
	AuditoriaLeer      = "auditoria.leer"
	// Pacientes (RF-01, RF-02)
	PacientesLeer     = "pacientes.leer"
	PacientesEscribir = "pacientes.escribir"
	AnamnesisEscribir = "anamnesis.escribir"
	// Clínica (RF-03 a RF-06, RN-01)
	ClinicaLeer              = "clinica.leer"
	ClinicaEscribir          = "clinica.escribir"
	ClinicaFirmar            = "clinica.firmar"
	ConsentimientosGestionar = "consentimientos.gestionar"
	RecetasEscribir          = "recetas.escribir"
	// Presupuestos (RF-07, RF-08)
	PresupuestosLeer     = "presupuestos.leer"
	PresupuestosEscribir = "presupuestos.escribir"
	PresupuestosAprobar  = "presupuestos.aprobar"
	// Agenda (RF-09 a RF-11, RN-03)
	CitasLeer     = "citas.leer"
	CitasEscribir = "citas.escribir"
	// Caja y facturación (RF-12, RN-02)
	CajaOperar        = "caja.operar"
	CajaRevisar       = "caja.revisar"
	FacturacionEmitir = "facturacion.emitir"
	// Inventario (RF-14)
	InventarioLeer       = "inventario.leer"
	InventarioEscribir   = "inventario.escribir"
	InventarioConfigurar = "inventario.configurar"
	// Honorarios (RF-13)
	HonorariosPropios    = "honorarios.propios"
	HonorariosTodos      = "honorarios.todos"
	HonorariosConfigurar = "honorarios.configurar"
)

// Todos los permisos, en orden (para la matriz documentada).
var Todos = []string{
	UsuariosGestionar, AjustesGestionar, CatalogosGestionar, AuditoriaLeer,
	PacientesLeer, PacientesEscribir, AnamnesisEscribir,
	ClinicaLeer, ClinicaEscribir, ClinicaFirmar, ConsentimientosGestionar, RecetasEscribir,
	PresupuestosLeer, PresupuestosEscribir, PresupuestosAprobar,
	CitasLeer, CitasEscribir,
	CajaOperar, CajaRevisar, FacturacionEmitir,
	InventarioLeer, InventarioEscribir, InventarioConfigurar,
	HonorariosPropios, HonorariosTodos, HonorariosConfigurar,
}

// Roles del SRS (sección 3).
const (
	AdminTI       = "ADMIN_TI"
	Recepcionista = "RECEPCIONISTA"
	Odontologo    = "ODONTOLOGO"
	Asistente     = "ASISTENTE"
	Auditor       = "AUDITOR"
)

var Roles = []string{AdminTI, Recepcionista, Odontologo, Asistente, Auditor}

var EtiquetaRol = map[string]string{
	AdminTI:       "Administrador TI / Sistema",
	Recepcionista: "Recepcionista / Cajero",
	Odontologo:    "Odontólogo General / Especialista",
	Asistente:     "Asistente Dental",
	Auditor:       "Auditor / Contador",
}

// Matriz rol → permisos (principio de menor privilegio).
var PorRol = map[string][]string{
	AdminTI: {UsuariosGestionar, AjustesGestionar, CatalogosGestionar, AuditoriaLeer},
	Recepcionista: {PacientesLeer, PacientesEscribir, PresupuestosLeer, PresupuestosAprobar,
		CitasLeer, CitasEscribir, CajaOperar, FacturacionEmitir},
	Odontologo: {PacientesLeer, PacientesEscribir, AnamnesisEscribir, ClinicaLeer, ClinicaEscribir,
		ClinicaFirmar, ConsentimientosGestionar, RecetasEscribir, PresupuestosLeer, PresupuestosEscribir,
		PresupuestosAprobar, CitasLeer, CitasEscribir, InventarioLeer, HonorariosPropios},
	Asistente: {PacientesLeer, AnamnesisEscribir, ClinicaLeer, ClinicaEscribir, CitasLeer,
		InventarioLeer, InventarioEscribir},
	Auditor: {CajaRevisar, InventarioLeer, InventarioConfigurar, HonorariosTodos, HonorariosConfigurar},
}

func Tiene(rol, permiso string) bool { return slices.Contains(PorRol[rol], permiso) }

func RolValido(rol string) bool { return slices.Contains(Roles, rol) }
