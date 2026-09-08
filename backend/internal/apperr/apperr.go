// Package apperr provides a small typed error used by the service layer to
// signal an outcome category (not found / conflict / invalid ...) without
// depending on the HTTP layer.
package apperr

import "fmt"

// Kind categorises a service-layer error.
type Kind int

const (
	Invalid       Kind = iota // -> 400 bad_request
	NotFound                  // -> 404 not_found
	Conflict                  // -> 409 conflict
	Unprocessable             // -> 422 unprocessable
)

// E is a categorised error.
type E struct {
	Kind Kind
	Msg  string
}

func (e *E) Error() string { return e.Msg }

// Invalidf builds an Invalid error.
func Invalidf(format string, a ...any) *E { return &E{Invalid, fmt.Sprintf(format, a...)} }

// NotFoundf builds a NotFound error.
func NotFoundf(format string, a ...any) *E { return &E{NotFound, fmt.Sprintf(format, a...)} }

// Conflictf builds a Conflict error.
func Conflictf(format string, a ...any) *E { return &E{Conflict, fmt.Sprintf(format, a...)} }

// Unprocessablef builds an Unprocessable error.
func Unprocessablef(format string, a ...any) *E { return &E{Unprocessable, fmt.Sprintf(format, a...)} }
