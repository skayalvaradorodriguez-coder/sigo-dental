// Package models contiene las estructuras que viajan por la API y el error estándar.
package models

// APIError es la forma ÚNICA de responder errores.
// `Code` es estable para que el frontend reaccione (p. ej. SESSION_EXPIRED → volver al login).
// `Error` repite `Message` por compatibilidad (mismo contrato que MultiCatálogo).
type APIError struct {
	Status  int            `json:"status"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Error   string         `json:"error"`
	Extra   map[string]any `json:"extra,omitempty"`
	Details string         `json:"details,omitempty"` // sólo en APP_ENV=dev
}

func NewAPIError(status int, code, message string) APIError {
	return APIError{Status: status, Code: code, Message: message, Error: message}
}

func (e APIError) Con(clave string, valor any) APIError {
	if e.Extra == nil {
		e.Extra = map[string]any{}
	}
	e.Extra[clave] = valor
	return e
}

// Pagina es el formato de todos los listados paginados.
type Pagina[T any] struct {
	Data  []T `json:"data"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}
