package permisos

import (
	"slices"
	"testing"
)

func TestMatrizUsaSoloPermisosDelCatalogo(t *testing.T) {
	for rol, ps := range PorRol {
		for _, p := range ps {
			if !slices.Contains(Todos, p) {
				t.Errorf("%s usa permiso desconocido %s", rol, p)
			}
		}
	}
}

func TestMenorPrivilegio(t *testing.T) {
	for _, p := range []string{PacientesLeer, ClinicaLeer, CajaOperar, HonorariosTodos} {
		if Tiene(AdminTI, p) {
			t.Errorf("ADMIN_TI no debe tener %s", p)
		}
	}
	for rol := range PorRol {
		if Tiene(rol, ClinicaFirmar) != (rol == Odontologo) {
			t.Errorf("sólo el odontólogo firma; %s", rol)
		}
	}
	if !Tiene(Recepcionista, CajaOperar) || Tiene(Recepcionista, CajaRevisar) {
		t.Error("cierre ciego: recepción opera caja pero no revisa arqueos")
	}
	if Tiene(Auditor, ClinicaLeer) || Tiene(Auditor, PacientesLeer) {
		t.Error("el auditor no accede a la historia clínica")
	}
}
