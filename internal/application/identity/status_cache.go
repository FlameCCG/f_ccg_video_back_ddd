package identity

import (
	"context"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// UserStatusCache 用户状态缓存端口（Redis cache-aside + generation CAS）。
type UserStatusCache interface {
	// Load 读取缓存中的用户状态；未命中返回 (0, false)。
	Load(ctx context.Context, userID uint) (enum.UserStatus, bool)
	// Generation 读取当前 generation 快照（读库前捕获，用于 CAS 回填）。
	Generation(ctx context.Context, userID uint) int64
	// StoreIfGenerationMatch 仅在 generation 未变化时回填状态。
	StoreIfGenerationMatch(ctx context.Context, userID uint, status enum.UserStatus, expectedGen int64)
	// Write 权威写穿：直接写入最新状态并 bump generation。
	Write(userID uint, status enum.UserStatus)
	// Invalidate 删除缓存并 bump generation。
	Invalidate(userID uint)
}
