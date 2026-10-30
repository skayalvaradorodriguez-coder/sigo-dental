package herramientas

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sigo-backend/config"
	"sigo-backend/seguridad"
)

// SesionDirecta crea una sesión con MFA verificado para un usuario, igual que la API tras el login
// (fila en `sesiones` + JWT con el mismo jti). Evita el límite de 10 logins/min al simular muchos usuarios.
func SesionDirecta(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, email string) (string, error) {
	var id, rol string
	if err := pool.QueryRow(ctx, `SELECT id, rol FROM usuarios WHERE email=$1 AND activo`, email).Scan(&id, &rol); err != nil {
		return "", fmt.Errorf("usuario %s: %w", email, err)
	}
	jti := seguridad.TokenAleatorio(24)
	expira := time.Now().Add(cfg.SesionMaxima)
	if _, err := pool.Exec(ctx, `INSERT INTO sesiones (jti, usuario_id, mfa_verificado, ip, user_agent, expira_en) VALUES ($1,$2,true,'127.0.0.1','sigo-herramientas',$3)`,
		jti, id, expira); err != nil {
		return "", err
	}
	return seguridad.GenerarToken(cfg.JWTSecret, id, email, rol, jti, expira)
}

type cliente struct {
	base, token string
	http        *http.Client
}

func (c *cliente) get(ruta string) (int, time.Duration, error) {
	req, _ := http.NewRequest("GET", c.base+ruta, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	t0 := time.Now()
	res, err := c.http.Do(req)
	if err != nil {
		return 0, time.Since(t0), err
	}
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
	return res.StatusCode, time.Since(t0), nil
}

func percentil(xs []time.Duration, p float64) time.Duration {
	if len(xs) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	k := int(float64(len(s)-1) * p)
	return s[k]
}

func ms(d time.Duration) string { return fmt.Sprintf("%.0f ms", float64(d.Microseconds())/1000) }

// MedirExpediente: para cada pestaña del expediente mide la ficha (GET /pacientes/:id) seguida de la
// llamada de la pestaña, como hace la interfaz al abrirla. Devuelve un informe en texto.
func MedirExpediente(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, base string, repeticiones int) (string, bool, error) {
	pid, err := Volumen(ctx, pool)
	if err != nil {
		return "", false, err
	}
	tok, err := SesionDirecta(ctx, pool, cfg, "odontologo@demo.sigo")
	if err != nil {
		return "", false, err
	}
	c := &cliente{base: strings.TrimRight(base, "/"), token: tok, http: &http.Client{Timeout: 30 * time.Second}}
	pestanas := []struct{ nombre, ruta string }{
		{"Ficha + anamnesis", "/api/pacientes/" + pid + "/anamnesis"},
		{"Ficha + odontograma", "/api/pacientes/" + pid + "/odontograma"},
		{"Ficha + periodontogramas", "/api/pacientes/" + pid + "/periodontogramas"},
		{"Ficha + evolución SOEP", "/api/pacientes/" + pid + "/notas"},
		{"Ficha + procedimientos", "/api/pacientes/" + pid + "/procedimientos"},
		{"Ficha + presupuestos", "/api/pacientes/" + pid + "/presupuestos"},
		{"Ficha + citas (±15 días)", "/api/citas?pacienteId=" + pid + "&desde=" + time.Now().AddDate(0, 0, -15).UTC().Format(time.RFC3339) + "&hasta=" + time.Now().AddDate(0, 0, 15).UTC().Format(time.RFC3339)},
	}
	// Calentamiento: la primera llamada abre conexiones y llena cachés.
	for _, p := range pestanas {
		_, _, _ = c.get("/api/pacientes/" + pid)
		_, _, _ = c.get(p.ruta)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "RNF-01 — expediente de un paciente con 10 años de historia (%s), %d repeticiones por pestaña\n", DocumentoVolumen, repeticiones)
	fmt.Fprintf(&b, "%-28s %9s %9s %9s %9s\n", "Pestaña", "p50", "p95", "máx", "tamaño")
	cumple := true
	for _, p := range pestanas {
		var tiempos []time.Duration
		var tam int
		for range repeticiones {
			t0 := time.Now()
			s1, _, e1 := c.get("/api/pacientes/" + pid)
			s2, _, e2 := c.get(p.ruta)
			if e1 != nil || e2 != nil || s1 != 200 || s2 != 200 {
				return "", false, fmt.Errorf("%s: estado %d/%d %v %v", p.nombre, s1, s2, e1, e2)
			}
			tiempos = append(tiempos, time.Since(t0))
		}
		if req, _ := http.NewRequest("GET", c.base+p.ruta, nil); req != nil {
			req.Header.Set("Authorization", "Bearer "+c.token)
			if res, err := c.http.Do(req); err == nil {
				body, _ := io.ReadAll(res.Body)
				_ = res.Body.Close()
				tam = len(body)
			}
		}
		p95 := percentil(tiempos, 0.95)
		if p95 > 1500*time.Millisecond {
			cumple = false
		}
		fmt.Fprintf(&b, "%-28s %9s %9s %9s %8.0f KB\n", p.nombre, ms(percentil(tiempos, 0.5)), ms(p95), ms(percentil(tiempos, 1)), float64(tam)/1024)
	}
	fmt.Fprintf(&b, "Criterio: p95 < 1 500 ms en cada pestaña (sólo servidor y red local; el navegador se mide aparte).\n")
	return b.String(), cumple, nil
}

// ---------------------------------------------------------------- RNF-03

type resultadoCarga struct {
	mu       sync.Mutex
	tiempos  map[string][]time.Duration
	errores  map[string]int
	total    int
	fallidas int
}

func (r *resultadoCarga) anotar(nombre string, d time.Duration, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tiempos[nombre] = append(r.tiempos[nombre], d)
	r.total++
	if !ok {
		r.errores[nombre]++
		r.fallidas++
	}
}

// Carga simula `usuarios` personas trabajando a la vez durante `duracion`: cada una repite un flujo
// realista (panel, agenda del día, búsqueda, ficha y expediente, heartbeat) con 1 a 3 s de pausa
// entre acciones, como una persona real. Criterio: p95 < 1,5 s y menos de 1 % de errores.
func Carga(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, base string, usuarios int, duracion time.Duration) (string, bool, error) {
	pid, err := Volumen(ctx, pool)
	if err != nil {
		return "", false, err
	}
	var demo string
	_ = pool.QueryRow(ctx, `SELECT id FROM pacientes WHERE numero_documento='DEMO000101'`).Scan(&demo)
	roles := []string{"recepcion@demo.sigo", "odontologo@demo.sigo", "odontologo2@demo.sigo", "asistente@demo.sigo"}
	tokens := make([]string, usuarios)
	for i := range tokens {
		if tokens[i], err = SesionDirecta(ctx, pool, cfg, roles[i%len(roles)]); err != nil {
			return "", false, err
		}
	}
	res := &resultadoCarga{tiempos: map[string][]time.Duration{}, errores: map[string]int{}}
	transporte := &http.Transport{MaxIdleConnsPerHost: usuarios, MaxConnsPerHost: 0}
	fin := time.Now().Add(duracion)
	hoy := time.Now()
	desde := time.Date(hoy.Year(), hoy.Month(), hoy.Day(), 0, 0, 0, 0, cfg.ZonaHoraria).UTC().Format(time.RFC3339)
	hasta := time.Date(hoy.Year(), hoy.Month(), hoy.Day(), 23, 59, 0, 0, cfg.ZonaHoraria).UTC().Format(time.RFC3339)
	var wg sync.WaitGroup
	for i := range usuarios {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := &cliente{base: strings.TrimRight(base, "/"), token: tokens[i], http: &http.Client{Timeout: 30 * time.Second, Transport: transporte}}
			rol := roles[i%len(roles)]
			paciente := pid
			if i%2 == 0 && demo != "" {
				paciente = demo
			}
			flujo := []struct{ nombre, ruta string }{
				{"panel", "/api/panel"},
				{"agenda del día", "/api/citas?desde=" + desde + "&hasta=" + hasta},
				{"búsqueda de pacientes", "/api/pacientes?q=a&limit=20"},
				{"ficha del paciente", "/api/pacientes/" + paciente},
			}
			if strings.HasPrefix(rol, "odontologo") || strings.HasPrefix(rol, "asistente") {
				flujo = append(flujo, struct{ nombre, ruta string }{"odontograma", "/api/pacientes/" + paciente + "/odontograma"},
					struct{ nombre, ruta string }{"notas SOEP", "/api/pacientes/" + paciente + "/notas"})
			} else {
				flujo = append(flujo, struct{ nombre, ruta string }{"catálogo de sillones", "/api/catalogos/sillones"})
			}
			pausa := func() time.Duration { // 1–3 s entre acciones, como una persona
				return time.Second + time.Duration(rand.IntN(2000))*time.Millisecond // #nosec G404 -- sólo espacia pedidos de la prueba de carga
			}
			time.Sleep(time.Duration(i%50) * 40 * time.Millisecond) // arranque escalonado (~2 s)
			for time.Now().Before(fin) {
				for _, paso := range flujo {
					if !time.Now().Before(fin) {
						return
					}
					st, d, err := c.get(paso.ruta)
					res.anotar(paso.nombre, d, err == nil && st == 200)
					time.Sleep(pausa())
				}
				st, d, err := c.post("/api/heartbeat")
				res.anotar("heartbeat", d, err == nil && st == 204)
			}
		}(i)
	}
	wg.Wait()
	var b strings.Builder
	var todos []time.Duration
	nombres := make([]string, 0, len(res.tiempos))
	for n, ts := range res.tiempos {
		nombres = append(nombres, n)
		todos = append(todos, ts...)
	}
	sort.Strings(nombres)
	fmt.Fprintf(&b, "RNF-03 — %d usuarios concurrentes durante %s (pausa de 1 a 3 s entre acciones)\n", usuarios, duracion)
	fmt.Fprintf(&b, "%-24s %8s %9s %9s %9s %8s\n", "Acción", "pedidos", "p50", "p95", "p99", "errores")
	for _, n := range nombres {
		ts := res.tiempos[n]
		fmt.Fprintf(&b, "%-24s %8d %9s %9s %9s %8d\n", n, len(ts), ms(percentil(ts, 0.5)), ms(percentil(ts, 0.95)), ms(percentil(ts, 0.99)), res.errores[n])
	}
	p95 := percentil(todos, 0.95)
	tasa := 0.0
	if res.total > 0 {
		tasa = float64(res.fallidas) / float64(res.total) * 100
	}
	fmt.Fprintf(&b, "TOTAL: %d pedidos (%.1f/s) · p50 %s · p95 %s · p99 %s · errores %.2f %%\n", res.total, float64(res.total)/duracion.Seconds(),
		ms(percentil(todos, 0.5)), ms(p95), ms(percentil(todos, 0.99)), tasa)
	cumple := p95 < 1500*time.Millisecond && tasa < 1
	fmt.Fprintf(&b, "Criterio: p95 < 1 500 ms y errores < 1 %%.\n")
	_, _ = pool.Exec(ctx, `UPDATE sesiones SET revocada_en=now(), motivo_revocacion='FIN_PRUEBA_CARGA' WHERE user_agent='sigo-herramientas' AND revocada_en IS NULL`)
	return b.String(), cumple, nil
}

func (c *cliente) post(ruta string) (int, time.Duration, error) {
	req, _ := http.NewRequest("POST", c.base+ruta, strings.NewReader(""))
	req.Header.Set("Authorization", "Bearer "+c.token)
	t0 := time.Now()
	res, err := c.http.Do(req)
	if err != nil {
		return 0, time.Since(t0), err
	}
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
	return res.StatusCode, time.Since(t0), nil
}
