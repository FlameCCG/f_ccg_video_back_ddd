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

func HasOwner(ctx context.Context) bool {
	uid, ok := ctx.Value(uploadOwnerKey{}).(uint)
	return ok && uid != 0
}
