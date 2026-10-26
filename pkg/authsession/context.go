// Package authsession carries a revocable authentication check for long-lived transports.
package authsession

import (
	"context"
	"errors"
)

type (
	key struct{}
	// Validator validates an authenticated session using the request context.
	Validator func(context.Context) error
)

// WithValidator attaches the session validator to the context.
func WithValidator(ctx context.Context, validate Validator) context.Context {
	return context.WithValue(ctx, key{}, validate)
}

// Copy copies the session validator to another context.
func Copy(dst, src context.Context) context.Context {
	if v, ok := src.Value(key{}).(Validator); ok {
		return WithValidator(dst, v)
	}

	return dst
}

// Validate validates the session when a validator is present.
func Validate(ctx context.Context) error {
	if v, ok := ctx.Value(key{}).(Validator); ok {
		return v(ctx)
	}

	return errors.New("authenticated session unavailable")
}
