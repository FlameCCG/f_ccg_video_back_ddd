package social

import "context"

// FocusRepository 关注关系聚合的写侧仓储（接口所有权归领域层）。
type FocusRepository interface {
	// Create 写入关注关系；唯一键冲突时返回 ErrAlreadyFocused。
	Create(ctx context.Context, focus *Focus) error
	// Delete 删除关注关系，返回受影响行数（0 表示原本未关注）。
	Delete(ctx context.Context, userID, focusUserID uint) (int64, error)
}
