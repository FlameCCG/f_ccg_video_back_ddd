// Package notification 实现通知记录写入、查询与已读更新。
//
// repository.go 实现 domain/notification 的仓储，actor_query.go 获取展示所需的操作者资料。
// 用户资料投影不属于通知写模型，角标缓存的维护由应用层在相应操作后协调。
//
// 仓储遵守消费方端口的不存在与错误约定。写入后返回恢复出的实体，不回填调用方持有的值。
package notification

import (
	"context"
	"errors"
	"time"

	notificationdomain "f_ccg_video_back_ddd/internal/domain/notification"
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

var _ notificationdomain.Repository = (*Repository)(nil)

// NewRepository 构造通知仓储。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) toDomain(row *po.UserGlobalNotification) *notificationdomain.Notification {
	if row == nil {
		return nil
	}

	return notificationdomain.RestoreNotification(notificationdomain.Snapshot{
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
	})
}

func (r *Repository) toPO(notice *notificationdomain.Notification) *po.UserGlobalNotification {
	if notice == nil {
		return nil
	}

	snap := notice.Snapshot()

	return &po.UserGlobalNotification{
		Model: po.Model{
			ID:        snap.ID,
			CreatedAt: snap.CreatedAt,
			UpdatedAt: snap.UpdatedAt,
		},
		Type:             snap.Type,
		ReceiverID:       snap.ReceiverID,
		ActionUserID:     snap.ActionUserID,
		ActionUserAvatar: snap.ActionUserAvatar,
		ActionUserName:   snap.ActionUserName,
		Title:            snap.Title,
		Content:          snap.Content,
		Link:             snap.Link,
		VideoID:          snap.VideoID,
		VideoTitle:       snap.VideoTitle,
		DynamicID:        snap.DynamicID,
		CommentID:        snap.CommentID,
	}
}

func (r *Repository) toDomainList(rows []po.UserGlobalNotification) []notificationdomain.Notification {
	out := make([]notificationdomain.Notification, 0, len(rows))
	for i := range rows {
		out = append(out, *r.toDomain(&rows[i]))
	}

	return out
}

// Create 写入通知并返回带自增 ID 与时间戳的实体。
func (r *Repository) Create(ctx context.Context, notice *notificationdomain.Notification) (*notificationdomain.Notification, error) {
	row := r.toPO(notice)
	if err := gormx.DB(ctx, r.db).Create(row).Error; err != nil {
		return nil, err
	}

	return r.toDomain(row), nil
}

// CreateBatch 批量写入通知，失败时整批不生效。
func (r *Repository) CreateBatch(ctx context.Context, notices []notificationdomain.Notification) ([]notificationdomain.Notification, error) {
	if len(notices) == 0 {
		return nil, nil
	}

	rows := make([]po.UserGlobalNotification, 0, len(notices))
	for i := range notices {
		rows = append(rows, *r.toPO(&notices[i]))
	}

	if err := gormx.DB(ctx, r.db).Create(&rows).Error; err != nil {
		return nil, err
	}

	return r.toDomainList(rows), nil
}

// FindByID 按主键读取通知；不存在时返回 (nil, nil)。
func (r *Repository) FindByID(ctx context.Context, id uint) (*notificationdomain.Notification, error) {
	var row po.UserGlobalNotification
	if err := gormx.DB(ctx, r.db).First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return r.toDomain(&row), nil
}

// ListForReceiver 按接收者与类型集合分页查询（含全员广播 -1）。
func (r *Repository) ListForReceiver(
	ctx context.Context,
	receiverID int,
	types []enum.Type,
	page, pageSize int,
) ([]notificationdomain.Notification, int64, error) {
	db := gormx.DB(ctx, r.db).Model(&po.UserGlobalNotification{}).
		Where("receiver_id IN (?, ?)", receiverID, notificationdomain.BroadcastReceiverID).
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

	return r.toDomainList(list), total, nil
}

// FindByIDsForUpdate 在调用方事务内锁定最新状态，避免删除资格与实际删除之间被管理端改写。
func (r *Repository) FindByIDsForUpdate(ctx context.Context, ids []uint) ([]notificationdomain.Notification, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var rows []po.UserGlobalNotification
	if err := gormx.DB(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", ids).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return r.toDomainList(rows), nil
}

// ListSystem 分页查询系统通知（管理端）。
func (r *Repository) ListSystem(ctx context.Context, page, pageSize int) ([]notificationdomain.Notification, int64, error) {
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

	return r.toDomainList(list), total, nil
}

// VisibleIDs 过滤出对该接收者可见（个人或全员）的通知 ID。
func (r *Repository) VisibleIDs(ctx context.Context, receiverID int, ids []uint) ([]uint, error) {
	var validIDs []uint
	if err := gormx.DB(ctx, r.db).
		Model(&po.UserGlobalNotification{}).
		Where("id IN ? AND receiver_id IN (?, ?)", ids, receiverID, notificationdomain.BroadcastReceiverID).
		Pluck("id", &validIDs).Error; err != nil {
		return nil, err
	}

	return validIDs, nil
}

// DeleteForReceiver 删除接收者的通知及其已读记录（同一事务）。
func (r *Repository) DeleteForReceiver(ctx context.Context, receiverID int, ids []uint) error {
	return gormx.NewTxManager(r.db).Do(ctx, func(txCtx context.Context) error {
		tx := gormx.DB(txCtx, r.db)
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
	return gormx.NewTxManager(r.db).Do(ctx, func(txCtx context.Context) error {
		tx := gormx.DB(txCtx, r.db)
		if err := tx.Where("id IN ?", ids).
			Delete(&po.UserGlobalNotification{}).Error; err != nil {
			return err
		}

		return tx.Where("notification_id IN ?", ids).
			Delete(&po.NotificationUserRead{}).Error
	})
}

// UpdateSystemContent 管理端更新系统通知内容，返回更新前的接收者。
func (r *Repository) UpdateSystemContent(ctx context.Context, notice *notificationdomain.Notification) (int, error) {
	if notice == nil || notice.ID() == 0 {
		return 0, gorm.ErrRecordNotFound
	}

	snap := notice.Snapshot()
	var previousReceiverID int
	err := gormx.NewTxManager(r.db).Do(ctx, func(txCtx context.Context) error {
		db := gormx.DB(txCtx, r.db)
		var old po.UserGlobalNotification
		// 即使调用方之前建立过一致性读快照，也必须以当前锁定读取得接收者。
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "receiver_id").First(&old, snap.ID).Error; err != nil {
			return err
		}
		previousReceiverID = old.ReceiverID
		return db.Model(&po.UserGlobalNotification{}).Where("id = ?", snap.ID).Updates(map[string]any{
			"title":       snap.Title,
			"content":     snap.Content,
			"link":        snap.Link,
			"receiver_id": snap.ReceiverID,
			"video_id":    snap.VideoID,
			"video_title": snap.VideoTitle,
			"type":        enum.NotificationSystemType,
		}).Error
	})
	return previousReceiverID, err
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
	types := (notificationdomain.TypePolicy{}).AllTypes()
	result := make(map[enum.Type]int64, len(types))

	type row struct {
		Type  enum.Type `gorm:"column:type"`
		Total int64     `gorm:"column:total"`
	}

	var rows []row
	if err := gormx.DB(ctx, r.db).
		Table("user_global_notifications AS n").
		Select("n.type, COUNT(*) AS total").
		Joins("LEFT JOIN notification_user_reads AS r ON r.notification_id = n.id AND r.user_id = ?", receiverID).
		Where("n.receiver_id IN (?, ?)", receiverID, notificationdomain.BroadcastReceiverID).
		Where("r.notification_id IS NULL").
		Where("n.type IN ?", types).
		Group("n.type").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, item := range rows {
		result[item.Type] = item.Total
	}

	return result, nil
}
