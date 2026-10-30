// Package database contiene las migraciones SQL (embebidas en el binario) y el migrador.
package database

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var archivos embed.FS

// Migrar aplica en orden las migraciones aún no registradas en `schema_migraciones`.
// Cada archivo es además idempotente: puede ejecutarse a mano con psql sin romper nada.
func Migrar(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migraciones (
		archivo TEXT PRIMARY KEY, aplicada_en TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return nil, err
	}
	entradas, err := archivos.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var nombres []string
	for _, e := range entradas {
		if strings.HasSuffix(e.Name(), ".sql") {
			nombres = append(nombres, e.Name())
		}
	}
	sort.Strings(nombres)
	var aplicadas []string
	for _, n := range nombres {
		var ya bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migraciones WHERE archivo=$1)`, n).Scan(&ya); err != nil {
			return aplicadas, err
		}
		if ya {
			continue
		}
		sql, _ := archivos.ReadFile(n)
		tx, err := pool.Begin(ctx)
		if err != nil {
			return aplicadas, err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return aplicadas, fmt.Errorf("%s: %w", n, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migraciones(archivo) VALUES ($1)`, n); err != nil {
			_ = tx.Rollback(ctx)
			return aplicadas, err
		}
		if err := tx.Commit(ctx); err != nil {
			return aplicadas, err
		}
		aplicadas = append(aplicadas, n)
	}
	return aplicadas, nil
}
