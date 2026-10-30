// SIGO-Dental — API REST (Go + Fiber). Comandos:
//
//	go run .                          inicia el servidor (PORT, por defecto 3000)
//	go run . migrar                   aplica las migraciones de database/*.sql
//	go run . semilla [--reset-usuarios]  carga datos de demostración (ficticios)
//	go run . totp <correo>            muestra el código MFA vigente de un usuario demo (no en producción)
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sigo-backend/almacenamiento"
	"sigo-backend/herramientas"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sigo-backend/cache"
	"sigo-backend/config"
	"sigo-backend/controllers"
	"sigo-backend/database"
	"sigo-backend/notificaciones"
	"sigo-backend/repository"
	"sigo-backend/routes"
	"sigo-backend/seguridad"
	"sigo-backend/semilla"
)

func main() {
	cfg, err := config.Cargar()
	if err != nil {
		log.Fatal(err)
	}
	// Comandos que no necesitan base de datos.
	if len(os.Args) > 1 && os.Args[1] == "openapi" {
		generarOpenAPI(cfg)
		return
	}
	ctx := context.Background()
	pool, err := config.ConectarDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("No se pudo conectar a PostgreSQL: %v\n¿Está corriendo la base (docker compose up -d) y es correcto DB_PORT?", err)
	}
	defer pool.Close()

	if len(os.Args) > 1 {
		ejecutarComando(ctx, cfg, pool, os.Args[1:])
		return
	}

	// Inyección de dependencias: interfaces en los bordes, implementaciones aquí.
	repo := repository.NuevoPostgres(pool)
	redis := cache.NuevaRedisCache(config.ConectarRedis(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB))
	despachador := &notificaciones.Despachador{Proveedor: &notificaciones.Simulada{}, Clinica: cfg.NombreClinica, Zona: cfg.ZonaHoraria}
	ctl := controllers.NewControllers(repo, redis, cfg, despachador)
	ctl.Archivos = &almacenamiento.Local{Dir: cfg.ArchivosDir}
	app, _ := routes.NuevaApp(ctl)

	// Goroutine de recordatorios con apagado ordenado (WaitGroup).
	ctxBg, cancelar := context.WithCancel(ctx)
	var wg sync.WaitGroup
	despachador.Iniciar(ctxBg, &wg, cfg.DespachoCada, ctl.ProcesarNotificaciones)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		log.Println("Apagando…")
		cancelar()
		_ = app.ShutdownWithTimeout(10 * time.Second)
	}()
	log.Printf("SIGO-Dental API en http://localhost:%s/api (entorno %s, zona %s)", cfg.Puerto, cfg.Entorno, cfg.NombreZona)
	if err := app.Listen(":" + cfg.Puerto); err != nil {
		log.Println(err)
	}
	cancelar()
	wg.Wait()
}

func ejecutarComando(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, args []string) {
	switch args[0] {
	case "migrar":
		ap, err := database.Migrar(ctx, pool)
		if err != nil {
			log.Fatalf("✖ Error al migrar: %v", err)
		}
		if len(ap) == 0 {
			fmt.Println("✔ La base ya estaba al día")
		}
		for _, a := range ap {
			fmt.Println("✔ Aplicada", a)
		}
	case "semilla":
		if cfg.EsProduccion() {
			log.Fatal("La semilla de demostración no se ejecuta en producción.")
		}
		reset := len(args) > 1 && args[1] == "--reset-usuarios"
		if err := semilla.Cargar(ctx, pool, reset, cfg.ZonaHoraria); err != nil {
			log.Fatalf("✖ Error en la semilla: %v", err)
		}
		fmt.Println("✔ Datos de demostración (ficticios) cargados. Usuarios:")
		for _, u := range semilla.Usuarios {
			extra := ""
			if u.PrimerIngreso {
				extra = "  (primer ingreso)"
			}
			fmt.Printf("   %-14s %s%s\n", u.Rol, u.Email, extra)
		}
		fmt.Println("  Contraseña:", semilla.PasswordDemo)
		fmt.Println("  Código MFA:  go run . totp <correo>")
	case "totp":
		if cfg.EsProduccion() {
			log.Fatal("No disponible en producción.")
		}
		if len(args) < 2 {
			log.Fatal("Indique el correo: go run . totp usuario@demo.sigo")
		}
		var sec *string
		if err := pool.QueryRow(ctx, `SELECT mfa_secreto FROM usuarios WHERE email=$1`, strings.ToLower(args[1])).Scan(&sec); err != nil || sec == nil {
			log.Fatal("Usuario inexistente o sin MFA configurado.")
		}
		resta := seguridad.PeriodoTOTP - time.Now().Unix()%seguridad.PeriodoTOTP
		fmt.Printf("%s   (válido ~%d s más)\n", seguridad.CodigoTOTP(*sec, time.Now()), resta)
	case "crear-admin":
		// Primer administrador en una base nueva (también en producción, donde no hay semilla).
		// Contraseña temporal que se muestra una vez; en el primer ingreso se cambia y se enrola el MFA.
		if len(args) < 3 {
			log.Fatal(`Uso: go run . crear-admin admin@clinica.com "Nombre Apellido"`)
		}
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM usuarios WHERE rol='ADMIN_TI' AND activo`).Scan(&n); err != nil {
			log.Fatal(err)
		}
		if n > 0 {
			log.Fatal("Ya existe un administrador activo: cree los demás usuarios desde la aplicación.")
		}
		temp := seguridad.PasswordTemporal()
		hash, err := seguridad.HashPassword(temp)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO usuarios (email, nombre, rol, password_hash, debe_cambiar_password, mfa_activo) VALUES ($1,$2,'ADMIN_TI',$3,true,false)`,
			strings.ToLower(strings.TrimSpace(args[1])), strings.TrimSpace(args[2]), hash); err != nil {
			log.Fatalf("No se pudo crear: %v", err)
		}
		fmt.Printf("✔ Administrador creado: %s\n  Contraseña temporal (se muestra una sola vez): %s\n  Al ingresar deberá cambiarla y configurar el MFA.\n", args[1], temp)
	case "token":
		// Sesión ya verificada para pruebas automáticas del navegador (Playwright). Nunca en producción.
		if cfg.EsProduccion() {
			log.Fatal("No disponible en producción.")
		}
		if len(args) < 2 {
			log.Fatal("Uso: go run . token odontologo@demo.sigo")
		}
		tok, err := herramientas.SesionDirecta(ctx, pool, cfg, strings.ToLower(args[1]))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(tok)
	case "volumen", "medir", "carga":
		if cfg.EsProduccion() {
			log.Fatal("Las herramientas de medición no se ejecutan en producción.")
		}
		base := "http://localhost:" + cfg.Puerto
		if len(args) > 1 {
			base = args[1]
		}
		switch args[0] {
		case "volumen":
			id, err := herramientas.Volumen(ctx, pool)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("✔ Paciente de volumen %s listo (id %s)\n", herramientas.DocumentoVolumen, id)
		case "medir":
			rep := 30
			if len(args) > 2 {
				_, _ = fmt.Sscan(args[2], &rep)
			}
			informe, ok, err := herramientas.MedirExpediente(ctx, pool, cfg, base, rep)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Print(informe)
			fmt.Println(map[bool]string{true: "✔ CUMPLE", false: "✖ NO CUMPLE"}[ok])
		case "carga":
			usuarios, segundos := 150, 60
			if len(args) > 2 {
				_, _ = fmt.Sscan(args[2], &usuarios)
			}
			if len(args) > 3 {
				_, _ = fmt.Sscan(args[3], &segundos)
			}
			informe, ok, err := herramientas.Carga(ctx, pool, cfg, base, usuarios, time.Duration(segundos)*time.Second)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Print(informe)
			fmt.Println(map[bool]string{true: "✔ CUMPLE", false: "✖ NO CUMPLE"}[ok])
		}
	default:
		log.Fatalf("Comando desconocido: %s (use migrar, semilla, totp, openapi, crear-admin, volumen, medir o carga)", args[0])
	}
}

// generarOpenAPI escribe ../docs/openapi.json a partir del registro real de rutas.
func generarOpenAPI(cfg config.Config) {
	_, registro := routes.NuevaApp(controllers.NewControllers(nil, nil, cfg, nil))
	doc, err := routes.OpenAPI(registro)
	if err != nil {
		log.Fatal(err)
	}
	destino := "../docs/openapi.json"
	if len(os.Args) > 2 {
		destino = os.Args[2]
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o750); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(destino, append(doc, '\n'), 0o600); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✔ %s (%d rutas)\n", destino, len(registro))
}
