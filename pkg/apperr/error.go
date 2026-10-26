// Package apperr preserves business errors and causes without transport or logging dependencies.
package apperr

import (
	"errors"
	"strings"
)

// Kind is a semantic error category independent of transport and logging levels.
type Kind string

// Standard semantic categories used by processing boundaries.
const (
	Validation  Kind = "validation"
	Rejected    Kind = "rejected"
	NotFound    Kind = "not_found"
	Conflict    Kind = "conflict"
	RateLimited Kind = "rate_limited"
	Canceled    Kind = "canceled"
	Degraded    Kind = "degraded"
	Security    Kind = "security"
	Dependency  Kind = "dependency"
	Internal    Kind = "internal"
)

// Error unwraps both branches, so errors.Is/As can inspect the business error and cause.
type Error struct {
	Operation string
	Code      string
	Kind      Kind
	Business  error
	Cause     error
}

func (e *Error) Error() string {
	var parts []string
	if e.Operation != "" {
		parts = append(parts, e.Operation)
	}

	if e.Business != nil {
		parts = append(parts, e.Business.Error())
	}

	if e.Cause != nil {
		parts = append(parts, e.Cause.Error())
	}

	return strings.Join(parts, ": ")
}

func (e *Error) Unwrap() []error {
	var out []error
	if e.Business != nil {
		out = append(out, e.Business)
	}

	if e.Cause != nil {
		out = append(out, e.Cause)
	}

	return out
}

// Wrap keeps the original error even when a safe business message replaces it.
// Empty kind leaves classification to the business descriptor at the boundary.
func Wrap(operation string, business, cause error, kind ...Kind) error {
	if cause == nil && business != nil && len(kind) == 0 {
		return business
	}

	e := &Error{Operation: operation, Business: business, Cause: cause}
	if len(kind) > 0 {
		e.Kind = kind[0]
	}

	return e
}

// Semantic returns the first explicit classification, walking outer wrappers first.
func Semantic(err error) *Error {
	if err == nil {
		return nil
	}
	//nolint:errorlint // Inspect this exact wrapper before traversing its causal branch.
	if e, ok := err.(*Error); ok {
		if e.Kind != "" {
			return e
		}

		return Semantic(e.Cause)
	}
	// Joined branches have no single outer semantic authority; boundaries classify each branch.
	if _, ok := err.(interface{ Unwrap() []error }); ok {
		return nil
	}

	return Semantic(errors.Unwrap(err))
}
