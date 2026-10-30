package notificaciones

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
)

// Envio: datos de un mensaje vencido (el repositorio los copia aquí; así no hay ciclo de paquetes).
type Envio struct {
	ID, Tipo, Canal, Nombres, Odontologo, Sede, EstadoCita string
	Destinatario                                           *string
	Inicio                                                 time.Time
	Intentos                                               int
}

type Resultado struct {
	Texto, Proveedor, IDProveedor string
	Simulado                      bool
	Err                           error
}

// Despachador envía los mensajes vencidos en segundo plano (goroutine con ticker).
type Despachador struct {
	Proveedor Mensajeria
	Clinica   string
	Zona      *time.Location
	mu        sync.Mutex
}

// Armar construye el texto y llama al proveedor. Se usa desde ProcesarVencidas.
func (d *Despachador) Armar(ctx context.Context, e Envio) Resultado {
	texto := Plantilla(e.Tipo, DatosPlantilla{
		Nombre:     strings.Fields(e.Nombres + " ")[0],
		Clinica:    d.Clinica,
		Sede:       e.Sede,
		Odontologo: strings.TrimSpace(strings.ReplaceAll(e.Odontologo, "(ficticio)", "")),
		Inicio:     e.Inicio,
		Zona:       d.Zona,
	})
	dest := ""
	if e.Destinatario != nil {
		dest = *e.Destinatario
	}
	id, err := d.Proveedor.Enviar(ctx, Mensaje{Canal: e.Canal, Destinatario: dest, Texto: texto})
	return Resultado{Texto: texto, Proveedor: d.Proveedor.Nombre(), IDProveedor: id, Simulado: d.Proveedor.Simulado(), Err: err}
}

// Iniciar lanza la goroutine. Se detiene al cancelar `ctx`; `wg` permite esperar a que termine
// el lote en curso durante el apagado ordenado.
func (d *Despachador) Iniciar(ctx context.Context, wg *sync.WaitGroup, cada time.Duration, procesar func(context.Context) (int, int, error)) {
	if cada <= 0 {
		return
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(cada)
		defer t.Stop()
		log.Printf("[recordatorios] despachador cada %s (proveedor: %s, simulado=%v)", cada, d.Proveedor.Nombre(), d.Proveedor.Simulado())
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				d.mu.Lock()
				env, fal, err := procesar(ctx)
				d.mu.Unlock()
				if err != nil && ctx.Err() == nil {
					log.Printf("[recordatorios] error: %v", err)
				} else if env+fal > 0 {
					log.Printf("[recordatorios] enviados=%d fallidos=%d", env, fal)
				}
			}
		}
	}()
}
