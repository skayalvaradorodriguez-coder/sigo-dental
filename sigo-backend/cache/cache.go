// Package cache: patrón Cache-Aside sobre Redis. Todas las operaciones son nil-safe:
// si Redis no está disponible, la API consulta PostgreSQL directamente (degradación).
package cache

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheRepository interface {
	Obtener(ctx context.Context, clave string, destino any) bool
	Guardar(ctx context.Context, clave string, valor any, ttl time.Duration)
	Eliminar(ctx context.Context, claves ...string)
	EliminarPatron(ctx context.Context, patron string)
	Ping(ctx context.Context) error
	Disponible() bool
}

type RedisCache struct{ c *redis.Client }

func NuevaRedisCache(c *redis.Client) *RedisCache { return &RedisCache{c: c} }

func (r *RedisCache) Disponible() bool { return r != nil && r.c != nil }

func (r *RedisCache) Obtener(ctx context.Context, clave string, destino any) bool {
	if !r.Disponible() {
		return false
	}
	b, err := r.c.Get(ctx, clave).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(b, destino) == nil
}

func (r *RedisCache) Guardar(ctx context.Context, clave string, valor any, ttl time.Duration) {
	if !r.Disponible() {
		return
	}
	b, err := json.Marshal(valor)
	if err != nil {
		return
	}
	if err := r.c.Set(ctx, clave, b, ttl).Err(); err != nil {
		log.Printf("[cache] no se pudo guardar %s: %v", clave, err)
	}
}

func (r *RedisCache) Eliminar(ctx context.Context, claves ...string) {
	if !r.Disponible() || len(claves) == 0 {
		return
	}
	_ = r.c.Del(ctx, claves...).Err()
}

// EliminarPatron borra con SCAN (no bloquea Redis como KEYS).
func (r *RedisCache) EliminarPatron(ctx context.Context, patron string) {
	if !r.Disponible() {
		return
	}
	iter := r.c.Scan(ctx, 0, patron, 200).Iterator()
	var claves []string
	for iter.Next(ctx) {
		claves = append(claves, iter.Val())
	}
	r.Eliminar(ctx, claves...)
}

func (r *RedisCache) Ping(ctx context.Context) error {
	if !r.Disponible() {
		return redis.ErrClosed
	}
	return r.c.Ping(ctx).Err()
}

// Claves y tiempos de vida usados por la API.
const (
	ClaveCatalogos = "catalogos:" // + nombre (estructura, sillones, procedimientos, convenios, odontologos)
	ClaveAjuste    = "ajuste:"    // + clave
	TTLCatalogos   = 10 * time.Minute
	TTLAjustes     = 1 * time.Minute
)
