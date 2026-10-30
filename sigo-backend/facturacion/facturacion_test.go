package facturacion

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func doc() Documento {
	return Documento{Tipo: "FACTURA", Secuencial: 7, Emisor: "Clínica", Receptor: Receptor{Documento: "Cédula X", Nombre: "Paciente"},
		Lineas: []Linea{{Descripcion: "Consulta", Monto: "25.00"}}, Total: "25.00", Fecha: "2026-10-30T10:00:00Z"}
}

func TestProveedorSimulado(t *testing.T) {
	s := &Simulado{}
	if !s.Simulado() || s.Nombre() != "simulado" {
		t.Fatal("debe declararse simulado")
	}
	r, err := s.Emitir(context.Background(), doc())
	if err != nil || r.Estado != "AUTORIZADO" || r.Numero != "SIM-F-00000007" || !strings.HasPrefix(r.Autorizacion, "SIM") || !strings.Contains(r.Mensaje, "SIMULADO") {
		t.Fatalf("factura: %+v %v", r, err)
	}
	nc := doc()
	nc.Tipo = "NOTA_CREDITO"
	if r, _ := s.Emitir(context.Background(), nc); r.Estado != "RECHAZADO" {
		t.Error("nota de crédito sin factura de referencia")
	}
	nc.Referencia = "SIM-F-00000007"
	if r, _ := s.Emitir(context.Background(), nc); r.Estado != "AUTORIZADO" || !strings.HasPrefix(r.Numero, "SIM-NC-") {
		t.Errorf("nota de crédito: %+v", r)
	}
	vacio := doc()
	vacio.Lineas = nil
	if r, _ := s.Emitir(context.Background(), vacio); r.Estado != "RECHAZADO" {
		t.Error("documento sin líneas")
	}
	s.FallarSiguiente = true
	if _, err := s.Emitir(context.Background(), doc()); !errors.Is(err, ErrNoDisponible) {
		t.Error("simula caída del servicio")
	}
	if _, err := s.Emitir(context.Background(), doc()); err != nil {
		t.Error("sólo falla una vez")
	}
}
