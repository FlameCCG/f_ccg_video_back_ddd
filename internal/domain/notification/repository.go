package notification

import (
	"context"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Repository 通知聚合仓储（接口所有权归领域层）。
type Repository interface {
	// Create 写入通知并回填自增 ID。
	Create(ctx context.Context, notice *Notification) error
	// FindByID 按主键读取通知；不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, id uint) (*Notification, error)
	// ListForReceiver 按接收者与类型集合分页查询（含全员广播）。
	ListForReceiver(
		ctx context.Context,
		receiverID int,
		types []enum.Type,
		page, pageSize int,
	) ([]Notification, int64, error)
	// ListSystem 分页查询系统通知（管理端）。
	ListSystem(ctx context.Context, page, pageSize int) ([]Notification, int64, error)
	// VisibleIDs 过滤出对该接收者可见（个人或全员）的通知 ID。
	VisibleIDs(ctx context.Context, receiverID int, ids []uint) ([]uint, error)
	// CountUndeletable 统计给定 ID 中属于系统消息或全员广播的条数。
	CountUndeletable(ctx context.Context, ids []uint) (int64, error)
	// DeletableIDs 过滤出该接收者可删除的非系统通知 ID。
	DeletableIDs(ctx context.Context, receiverID int, ids []uint) ([]uint, error)
	// DeleteForReceiver 删除接收者的通知及其已读记录（同一事务）。
	DeleteForReceiver(ctx context.Context, receiverID int, ids []uint) error
	// DeleteByIDs 按 ID 删除通知及其已读记录（管理端，同一事务）。
	DeleteByIDs(ctx context.Context, ids []uint) error
	// DistinctReceivers 读取给定通知 ID 涉及的接收者集合。
	DistinctReceivers(ctx context.Context, ids []uint) ([]int, error)
	// UpdateSystemContent 管理端更新系统通知内容，返回更新前的接收者。
	UpdateSystemContent(ctx context.Context, notice *Notification) (previousReceiverID int, err error)
	// MarkRead 为接收者插入已读记录（冲突忽略）。
	MarkRead(ctx context.Context, receiverID int, ids []uint) error
	// ReadSet 读取用户在给定通知集合中的已读 ID。
	ReadSet(ctx context.Context, userID uint, ids []uint) (map[uint]struct{}, error)
	// CountUnreadGrouped 按类型统计未读数量。
	CountUnreadGrouped(ctx context.Context, receiverID int) (map[enum.Type]int64, error)
}
