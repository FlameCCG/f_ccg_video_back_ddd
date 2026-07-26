// Package notification 是通知上下文的持久化实现。
package notification

import (
	"context"
	"errors"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository 通知仓储的 GORM 实现。
type Repository struct {
	db *gorm.DB
}

var _ domain.Repository = (*Repository)(nil)

// NewRepository 构造通知仓储。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func toDomain(row *po.UserGlobalNotification) *domain.Notification {
	if row == nil {
		return nil
	}

	return &domain.Notification{
		ID:               row.ID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Type:             row.Type,
		ReceiverID:       row.ReceiverID,
		ActionUserID:     row.ActionUserID,
		ActionUserAvatar: row.ActionUserAvatar,
		ActionUserName:   row.ActionUserName,
		Title:            row.Title,
		Content:          row.Content,
		Link:             row.Link,
		VideoID:          row.VideoID,
		VideoTitle:       row.VideoTitle,
		DynamicID:        row.DynamicID,
		CommentID:        row.CommentID,
	}
}

func toPO(notice *domain.Notification) *po.UserGlobalNotification {
	if notice == nil {
		return nil
	}

	return &po.UserGlobalNotification{
		Model: po.Model{
			ID:        notice.ID,
			CreatedAt: notice.CreatedAt,
			UpdatedAt: notice.UpdatedAt,
		},
		Type:             notice.Type,
		ReceiverID:       notice.ReceiverID,
		ActionUserID:     notice.ActionUserID,
		ActionUserAvatar: notice.ActionUserAvatar,
		ActionUserName:   notice.ActionUserName,
		Title:            notice.Title,
		Content:          notice.Content,
		Link:             notice.Link,
		VideoID:          notice.VideoID,
		VideoTitle:       notice.VideoTitle,
		DynamicID:        notice.DynamicID,
		CommentID:        notice.CommentID,
	}
}

func toDomainList(rows []po.UserGlobalNotification) []domain.Notification {
	out := make([]domain.Notification, 0, len(rows))
	for i := range rows {
		out = append(out, *toDomain(&rows[i]))
	}

	return out
}

// Create 写入通知并回填自增 ID 与时间戳。
func (r *Repository) Create(ctx context.Context, notice *domain.Notification) error {
	row := toPO(notice)
	if err := gormx.DB(ctx, r.db).Create(row).Error; err != nil {
		return err
	}

	notice.ID = row.ID
	notice.CreatedAt = row.CreatedAt
	notice.UpdatedAt = row.UpdatedAt

	return nil
}

// FindByID 按主键读取通知；不存在时返回 (nil, nil)。
func (r *Repository) FindByID(ctx context.Context, id uint) (*domain.Notification, error) {
	var row po.UserGlobalNotification
	if err := gormx.DB(ctx, r.db).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomain(&row), nil
}

// ListForReceiver 按接收者与类型集合分页查询（含全员广播 -1）。
func (r *Repository) ListForReceiver(
	ctx context.Context,
	receiverID int,
	types []enum.Type,
	page, pageSize int,
) ([]domain.Notification, int64, error) {
	db := gormx.DB(ctx, r.db).Model(&po.UserGlobalNotification{}).
		Where("receiver_id IN (?, ?)", receiverID, domain.BroadcastReceiverID).
		Where("type IN ?", types)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []po.UserGlobalNotification
	if err := db.Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return toDomainList(list), total, nil
}

// ListSystem 分页查询系统通知（管理端）。
func (r *Repository) ListSystem(ctx context.Context, page, pageSize int) ([]domain.Notification, int64, error) {
	db := gormx.DB(ctx, r.db).Model(&po.UserGlobalNotification{})

	var total int64
	if err := db.Where("type = ?", enum.NotificationSystemType).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []po.UserGlobalNotification
	if err := db.Where("type = ?", enum.NotificationSystemType).
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return toDomainList(list), total, nil
}

// VisibleIDs 过滤出对该接收者可见（个人或全员）的通知 ID。
func (r *Repository) VisibleIDs(ctx context.Context, receiverID int, ids []uint) ([]uint, error) {
	var validIDs []uint
	if err := gormx.DB(ctx, r.db).
		Model(&po.UserGlobalNotification{}).
		Where("id IN ? AND receiver_id IN (?, ?)", ids, receiverID, domain.BroadcastReceiverID).
		Pluck("id", &validIDs).Error; err != nil {
		return nil, err
	}

	return validIDs, nil
}

// CountUndeletable 统计给定 ID 中属于系统消息或全员广播的条数。
func (r *Repository) CountUndeletable(ctx context.Context, ids []uint) (int64, error) {
	var blocked int64
	if err := gormx.DB(ctx, r.db).
		Model(&po.UserGlobalNotification{}).
		Where("id IN ?", ids).
		Where("(type = ? OR receiver_id = -1)", enum.NotificationSystemType).
		Count(&blocked).Error; err != nil {
		return 0, err
	}

	return blocked, nil
}

// DeletableIDs 过滤出该接收者可删除的非系统通知 ID。
func (r *Repository) DeletableIDs(ctx context.Context, receiverID int, ids []uint) ([]uint, error) {
	var deletableIDs []uint
	if err := gormx.DB(ctx, r.db).
		Model(&po.UserGlobalNotification{}).
		Where("id IN ? AND receiver_id = ? AND type <> ?", ids, receiverID, enum.NotificationSystemType).
		Pluck("id", &deletableIDs).Error; err != nil {
		return nil, err
	}

	return deletableIDs, nil
}

// DeleteForReceiver 删除接收者的通知及其已读记录（同一事务）。
func (r *Repository) DeleteForReceiver(ctx context.Context, receiverID int, ids []uint) error {
	return gormx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id IN ? AND receiver_id = ?", ids, receiverID).
			Delete(&po.UserGlobalNotification{}).Error; err != nil {
			return err
		}

		return tx.Where("notification_id IN ?", ids).
			Delete(&po.NotificationUserRead{}).Error
	})
}

// DeleteByIDs 按 ID 删除通知及其已读记录（管理端，同一事务）。
func (r *Repository) DeleteByIDs(ctx context.Context, ids []uint) error {
	return gormx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id IN ?", ids).
			Delete(&po.UserGlobalNotification{}).Error; err != nil {
			return err
		}

		return tx.Where("notification_id IN ?", ids).
			Delete(&po.NotificationUserRead{}).Error
	})
}

// DistinctReceivers 读取给定通知 ID 涉及的接收者集合。
func (r *Repository) DistinctReceivers(ctx context.Context, ids []uint) ([]int, error) {
	var receivers []int
	if err := gormx.DB(ctx, r.db).
		Model(&po.UserGlobalNotification{}).
		Where("id IN ?", ids).
		Distinct("receiver_id").
		Pluck("receiver_id", &receivers).Error; err != nil {
		return nil, err
	}

	return receivers, nil
}

// UpdateSystemContent 管理端更新系统通知内容，返回更新前的接收者。
func (r *Repository) UpdateSystemContent(ctx context.Context, notice *domain.Notification) (int, error) {
	if notice == nil || notice.ID == 0 {
		return 0, gorm.ErrRecordNotFound
	}

	db := gormx.DB(ctx, r.db)

	var old po.UserGlobalNotification
	if err := db.Select("id", "receiver_id").First(&old, notice.ID).Error; err != nil {
		return 0, err
	}

	if err := db.
		Model(&po.UserGlobalNotification{}).
		Where("id = ?", notice.ID).
		Updates(map[string]any{
			"title":       notice.Title,
			"content":     notice.Content,
			"link":        notice.Link,
			"receiver_id": notice.ReceiverID,
			"video_id":    notice.VideoID,
			"video_title": notice.VideoTitle,
			"type":        enum.NotificationSystemType,
		}).Error; err != nil {
		return 0, err
	}

	return old.ReceiverID, nil
}

// MarkRead 为接收者插入已读记录（冲突忽略）。
func (r *Repository) MarkRead(ctx context.Context, receiverID int, ids []uint) error {
	now := time.Now()

	rows := make([]po.NotificationUserRead, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, po.NotificationUserRead{
			UserID:         uint(receiverID),
			NotificationID: id,
			CreatedAt:      now,
		})
	}

	return gormx.DB(ctx, r.db).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&rows).Error
}

// ReadSet 读取用户在给定通知集合中的已读 ID。
func (r *Repository) ReadSet(ctx context.Context, userID uint, ids []uint) (map[uint]struct{}, error) {
	set := make(map[uint]struct{}, len(ids))
	if userID == 0 || len(ids) == 0 {
		return set, nil
	}

	var reads []po.NotificationUserRead
	if err := gormx.DB(ctx, r.db).
		Select("notification_id").
		Where("user_id = ? AND notification_id IN ?", userID, ids).
		Find(&reads).Error; err != nil {
		return nil, err
	}

	for _, read := range reads {
		set[read.NotificationID] = struct{}{}
	}

	return set, nil
}

// CountUnreadGrouped 按类型统计未读数量（未读 = 内容可见且无已读行）。
func (r *Repository) CountUnreadGrouped(ctx context.Context, receiverID int) (map[enum.Type]int64, error) {
	result := make(map[enum.Type]int64, len(domain.AllTypes()))

	type row struct {
		Type  enum.Type `gorm:"column:type"`
		Total int64     `gorm:"column:total"`
	}

	var rows []row
	if err := gormx.DB(ctx, r.db).
		Table("user_global_notifications AS n").
		Select("n.type, COUNT(*) AS total").
		Joins("LEFT JOIN notification_user_reads AS r ON r.notification_id = n.id AND r.user_id = ?", receiverID).
		Where("n.receiver_id IN (?, ?)", receiverID, domain.BroadcastReceiverID).
		Where("r.notification_id IS NULL").
		Where("n.type IN ?", domain.AllTypes()).
		Group("n.type").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, item := range rows {
		result[item.Type] = item.Total
	}

	return result, nil
}
