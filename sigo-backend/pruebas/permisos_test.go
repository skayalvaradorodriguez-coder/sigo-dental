package pruebas

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"sigo-backend/permisos"
)

// Matriz rol × endpoint derivada de permisos.PorRol (fuente única de verdad).
func TestMatrizDePermisos(t *testing.T) {
	requiereBD(t)
	desde := url.QueryEscape(time.Now().Format(time.RFC3339))
	hasta := url.QueryEscape(time.Now().Add(24 * time.Hour).Format(time.RFC3339))
	ana := idAna(t)
	endpoints := []struct {
		metodo, ruta string
		requiere     []string // basta con uno
	}{
		{"GET", "/api/usuarios", []string{permisos.UsuariosGestionar}},
		{"GET", "/api/usuarios/roles", []string{permisos.UsuariosGestionar}},
		{"GET", "/api/ajustes", []string{permisos.AjustesGestionar}},
		{"GET", "/api/auditoria", []string{permisos.AuditoriaLeer}},
		{"POST", "/api/auditoria/verificar", []string{permisos.AuditoriaLeer}},
		{"GET", "/api/pacientes", []string{permisos.PacientesLeer}},
		{"GET", "/api/citas?desde=" + desde + "&hasta=" + hasta, []string{permisos.CitasLeer}},
		{"GET", "/api/notificaciones", []string{permisos.CitasLeer}},
		{"GET", "/api/catalogos/estructura", []string{permisos.CatalogosGestionar, permisos.CitasLeer, permisos.PacientesLeer, permisos.InventarioLeer, permisos.HonorariosTodos}},
		{"GET", "/api/pacientes/" + ana + "/odontograma", []string{permisos.ClinicaLeer}},
		{"GET", "/api/pacientes/" + ana + "/notas", []string{permisos.ClinicaLeer}},
		{"GET", "/api/pacientes/" + ana + "/procedimientos", []string{permisos.ClinicaLeer}},
		{"GET", "/api/presupuestos", []string{permisos.PresupuestosLeer}},
		{"GET", "/api/pacientes/" + ana + "/tareas", []string{permisos.CitasLeer, permisos.PresupuestosLeer}},
		{"GET", "/api/consentimientos", []string{permisos.ConsentimientosGestionar}},
		{"GET", "/api/consentimientos/plantillas", []string{permisos.ConsentimientosGestionar, permisos.CatalogosGestionar}},
		{"GET", "/api/pacientes/" + ana + "/recetas", []string{permisos.ClinicaLeer}},
		{"GET", "/api/caja/sesion", []string{permisos.CajaOperar}},
		{"GET", "/api/pacientes/" + ana + "/por-cobrar", []string{permisos.CajaOperar}},
		{"GET", "/api/pacientes/" + ana + "/cobros", []string{permisos.CajaOperar, permisos.CajaRevisar}},
		{"GET", "/api/caja/cierres", []string{permisos.CajaRevisar}},
		{"GET", "/api/caja/cuentas-por-cobrar", []string{permisos.CajaRevisar}},
		{"GET", "/api/inventario/stock", []string{permisos.InventarioLeer}},
		{"GET", "/api/inventario/recetas", []string{permisos.InventarioLeer}},
		{"GET", "/api/honorarios/reglas", []string{permisos.HonorariosPropios, permisos.HonorariosTodos, permisos.HonorariosConfigurar}},
		{"GET", "/api/honorarios/liquidaciones", []string{permisos.HonorariosPropios, permisos.HonorariosTodos}},
	}
	usuarios := map[string]string{permisos.AdminTI: admin, permisos.Recepcionista: recepcion, permisos.Odontologo: odontologo,
		permisos.Asistente: asistente, permisos.Auditor: auditor}
	for _, rol := range permisos.Roles {
		tok := ingresar(t, usuarios[rol])
		for _, e := range endpoints {
			t.Run(fmt.Sprintf("%s %s %s", rol, e.metodo, e.ruta), func(t *testing.T) {
				permitido := false
				for _, p := range e.requiere {
					permitido = permitido || permisos.Tiene(rol, p)
				}
				want := 403
				if permitido {
					want = 200
				}
				espera(t, llamar(t, e.metodo, e.ruta, tok, nil), want, "acceso")
			})
		}
		w := llamar(t, "GET", "/api/panel", tok, nil).Body["widgets"].(map[string]any)
		_, tieneUsuarios := w["usuarios"]
		_, tieneSeguridad := w["seguridad"]
		_, tieneAgenda := w["agendaHoy"]
		if tieneUsuarios != permisos.Tiene(rol, permisos.UsuariosGestionar) || tieneSeguridad != permisos.Tiene(rol, permisos.AuditoriaLeer) ||
			tieneAgenda != permisos.Tiene(rol, permisos.CitasLeer) {
			t.Errorf("%s: el panel muestra indicadores que no corresponden: %v", rol, w)
		}
	}
}
