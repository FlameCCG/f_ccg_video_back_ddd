// Package uploadscope carries authenticated upload ownership and isolates chunk namespaces.
package uploadscope

import (
	"context"
	"fmt"
	"strings"
)

type uploadOwnerKey struct{}

// WithOwner isolates resumable staging data while completed content remains deduplicated.
func WithOwner(ctx context.Context, userID uint) context.Context {
	return context.WithValue(ctx, uploadOwnerKey{}, userID)
}

// ChunkNamespace isolates upload chunks by the authenticated owner when present.
func ChunkNamespace(ctx context.Context, hash string) string {
	if uid, ok := ctx.Value(uploadOwnerKey{}).(uint); ok && uid != 0 {
		prefix := fmt.Sprintf("u%d_", uid)
		if strings.HasPrefix(hash, prefix) {
			return hash
		}

		return prefix + hash
	}

	return hash
}

// HasOwner reports whether the context contains an authenticated upload owner.
func HasOwner(ctx context.Context) bool {
	uid, ok := ctx.Value(uploadOwnerKey{}).(uint)
	return ok && uid != 0
}
