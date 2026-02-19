package domain

import "errors"

// Sentinel errors for domain-level error discrimination.
// Services wrap these so handlers can map to HTTP status codes without leaking infrastructure details.
var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrBadRequest   = errors.New("bad request")
)
