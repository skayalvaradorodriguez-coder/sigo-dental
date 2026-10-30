// Package almacenamiento guarda el contenido de los archivos del paciente (radiografías, fotos,
// documentos). La interfaz permite cambiar el disco local por S3 o Azure Blob en producción sin
// tocar los controladores (el SRS sugiere S3/Azure; aquí no hay credenciales, por eso Local).
package almacenamiento

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

type Almacen interface {
	Guardar(ctx context.Context, clave string, datos []byte) error
	Leer(ctx context.Context, clave string) ([]byte, error)
}

// Local: carpeta en disco. Las claves son "paciente-uuid/archivo-uuid" (sin rutas arbitrarias).
type Local struct{ Dir string }

var claveValida = regexp.MustCompile(`^[0-9a-f-]{36}/[0-9a-f-]{36}$`)

func (l *Local) ruta(clave string) (string, error) {
	if !claveValida.MatchString(clave) {
		return "", fmt.Errorf("clave de almacenamiento inválida")
	}
	return filepath.Join(l.Dir, filepath.FromSlash(clave)), nil
}

func (l *Local) Guardar(_ context.Context, clave string, datos []byte) error {
	r, err := l.ruta(clave)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r), 0o750); err != nil {
		return err
	}
	if _, err := os.Stat(r); err == nil {
		return errors.New("el archivo ya existe: no se sobrescribe")
	}
	return os.WriteFile(r, datos, 0o600)
}

func (l *Local) Leer(_ context.Context, clave string) ([]byte, error) {
	r, err := l.ruta(clave)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(r) // #nosec G304 -- la ruta sale de una clave validada (uuid/uuid) dentro de Dir
}

// Memoria: para pruebas.
type Memoria struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (x *Memoria) Guardar(_ context.Context, clave string, datos []byte) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.m == nil {
		x.m = map[string][]byte{}
	}
	if _, ok := x.m[clave]; ok {
		return errors.New("el archivo ya existe: no se sobrescribe")
	}
	x.m[clave] = append([]byte(nil), datos...)
	return nil
}

func (x *Memoria) Leer(_ context.Context, clave string) ([]byte, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	d, ok := x.m[clave]
	if !ok {
		return nil, os.ErrNotExist
	}
	return d, nil
}

// Alterar: sólo para pruebas de integridad (simula un archivo modificado fuera del sistema).
func (x *Memoria) Alterar(clave string, datos []byte) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.m[clave] = datos
}
