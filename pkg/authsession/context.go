// Package authsession carries a revocable authentication check for long-lived transports.
package authsession

import (
	"context"
	"errors"
)

type key struct{}
type Validator func(context.Context) error

func WithValidator(ctx context.Context, validate Validator) context.Context {
	return context.WithValue(ctx, key{}, validate)
}
func Copy(dst, src context.Context) context.Context {
	if v, ok := src.Value(key{}).(Validator); ok {
		return WithValidator(dst, v)
	}
	return dst
}
func Validate(ctx context.Context) error {
	if v, ok := ctx.Value(key{}).(Validator); ok {
		return v(ctx)
	}
	return errors.New("authenticated session unavailable")
}
