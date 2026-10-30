package config

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// ConectarRedis devuelve nil si Redis no responde: la API sigue funcionando sin caché
// (degradación controlada, igual que en MultiCatálogo).
// db: número de base de Redis (0 para la aplicación; las pruebas usan otra para no mezclar cachés).
func ConectarRedis(addr, pass string, db int) *redis.Client {
	c := redis.NewClient(&redis.Options{Addr: addr, Password: pass, DB: db, DialTimeout: 2 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		log.Printf("[redis] sin conexión en %s (%v): se trabajará sin caché", addr, err)
		_ = c.Close()
		return nil
	}
	return c
}
