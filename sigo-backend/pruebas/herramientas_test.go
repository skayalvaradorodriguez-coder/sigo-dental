package pruebas

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"sigo-backend/herramientas"
)

// Etapa 7 — las herramientas de medición (RNF-01, RNF-03) funcionan contra la API real.
func TestHerramientasDeMedicion(t *testing.T) {
	requiereBD(t)
	ctx := context.Background()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(ln) }()
	base := "http://" + ln.Addr().String()

	id, err := herramientas.Volumen(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	var notas, sitios int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM notas_soep WHERE paciente_id=$1`, id).Scan(&notas)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM perio_sitios s JOIN periodontogramas g ON g.id = s.periodontograma_id WHERE g.paciente_id=$1`, id).Scan(&sitios)
	if notas != 200 || sitios != 20*28*6 {
		t.Errorf("volumen: %d notas, %d sitios", notas, sitios)
	}
	if id2, _ := herramientas.Volumen(ctx, pool); id2 != id {
		t.Error("el volumen se crea una sola vez")
	}
	inf, ok, err := herramientas.MedirExpediente(ctx, pool, ctl.Cfg, base, 3)
	if err != nil || !ok || !strings.Contains(inf, "Ficha + evolución SOEP") {
		t.Errorf("medición RNF-01: %v %v\n%s", ok, err, inf)
	}
	inf, ok, err = herramientas.Carga(ctx, pool, ctl.Cfg, base, 6, 3*time.Second)
	if err != nil || !ok || !strings.Contains(inf, "errores 0.00 %") {
		t.Errorf("carga RNF-03: %v %v\n%s", ok, err, inf)
	}
	var abiertas int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM sesiones WHERE user_agent='sigo-herramientas' AND revocada_en IS NULL AND usuario_id <> (SELECT id FROM usuarios WHERE email='odontologo@demo.sigo')`).Scan(&abiertas)
	if abiertas != 0 {
		t.Errorf("la prueba de carga revoca sus sesiones: quedan %d", abiertas)
	}
}
