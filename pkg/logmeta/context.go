// Package logmeta carries correlation values, never a logger, through contexts.
package logmeta

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// Values holds only explicit correlation identifiers.
type Values struct {
	RequestID    string
	UserID       string
	TaskID       string
	ParentTaskID string
	AttemptID    string
	ConnectionID string
	MessageID    string
}

type key struct{}

// From reads correlation values without retrieving a logger.
func From(ctx context.Context) Values {
	if ctx == nil {
		return Values{}
	}

	v, _ := ctx.Value(key{}).(Values)

	return v
}

// With attaches a new immutable metadata value.
func With(ctx context.Context, v Values) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, key{}, v)
}

// Copy preserves correlation when a context is replaced.
func Copy(dst, src context.Context) context.Context { return With(dst, From(src)) }

// NewID uses random server-side identifiers; caller supplied HTTP IDs are never reused.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}

	return hex.EncodeToString(b[:])
}
