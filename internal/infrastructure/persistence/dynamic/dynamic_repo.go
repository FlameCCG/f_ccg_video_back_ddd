package dynamic

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/dynamic"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// mysqlDuplicateEntryErrNo MySQL 唯一键冲突错误码。
const mysqlDuplicateEntryErrNo = 1062

// Repository 动态聚合仓储的 GORM 实现。
type Repository struct {
	db *gorm.DB
}

var _ domain.Repository = (*Repository)(nil)

// NewDynamicRepository 构造动态仓储。
func NewDynamicRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindByID 按主键读取动态；不存在时返回 (nil, nil)。
func (r *Repository) FindByID(ctx context.Context, dynamicID uint) (*domain.Dynamic, error) {
	var row po.Dynamic
	if err := gormx.DB(ctx, r.db).Where("id = ?", dynamicID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainDynamic(&row), nil
}

// ExistsByID 探测动态是否存在（列选择与原 ToggleLike 的 Take 一致）。
func (r *Repository) ExistsByID(ctx context.Context, dynamicID uint) (bool, error) {
	var row po.Dynamic
	if err := gormx.DB(ctx, r.db).
		Select("id", "user_id", "like_count").
		Take(&row, "id = ?", dynamicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// FindNoticeTargetByID 只读 id/user_id/content，供点赞通知渲染；不存在时返回 (nil, nil)。
func (r *Repository) FindNoticeTargetByID(ctx context.Context, dynamicID uint) (*domain.Dynamic, error) {
	var row po.Dynamic
	if err := gormx.DB(ctx, r.db).
		Select("id", "user_id", "content").
		Where("id = ?", dynamicID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainDynamic(&row), nil
}

// Create 写入动态，并把自增 ID 与时间戳回填到领域实体。
func (r *Repository) Create(ctx context.Context, d *domain.Dynamic) error {
	if d == nil {
		return nil
	}

	row := toPODynamic(d)
	if err := gormx.DB(ctx, r.db).Create(row).Error; err != nil {
		return err
	}

	d.ID = row.ID
	d.CreatedAt = row.CreatedAt
	d.UpdatedAt = row.UpdatedAt
	d.LikeCount = row.LikeCount
	d.IsPinned = row.IsPinned
	d.PinnedAt = row.PinnedAt

	return nil
}

// DeleteByID 删除单条动态。
func (r *Repository) DeleteByID(ctx context.Context, dynamicID uint) error {
	return gormx.DB(ctx, r.db).Where("id = ?", dynamicID).Delete(&po.Dynamic{}).Error
}

// DeleteByIDs 批量删除动态，返回受影响行数。
func (r *Repository) DeleteByIDs(ctx context.Context, dynamicIDs []uint) (int64, error) {
	res := gormx.DB(ctx, r.db).
		Where("id IN ?", dynamicIDs).
		Delete(&po.Dynamic{})
	if res.Error != nil {
		return 0, res.Error
	}

	return res.RowsAffected, nil
}

// ClearPinnedByUser 清除该用户当前已置顶的动态。
func (r *Repository) ClearPinnedByUser(ctx context.Context, userID uint) error {
	return gormx.DB(ctx, r.db).Model(&po.Dynamic{}).
		Where("user_id = ? AND is_pinned = ?", userID, true).
		Updates(map[string]any{"is_pinned": false, "pinned_at": nil}).Error
}

// SetPinned 把指定动态标记为置顶，并写入置顶时间。
func (r *Repository) SetPinned(ctx context.Context, dynamicID uint, pinnedAt *time.Time) error {
	return gormx.DB(ctx, r.db).Model(&po.Dynamic{}).
		Where("id = ?", dynamicID).
		Updates(map[string]any{"is_pinned": true, "pinned_at": pinnedAt}).Error
}

// ClearPinned 取消指定动态的置顶状态。
func (r *Repository) ClearPinned(ctx context.Context, dynamicID uint) error {
	return gormx.DB(ctx, r.db).Model(&po.Dynamic{}).
		Where("id = ?", dynamicID).
		Updates(map[string]any{"is_pinned": false, "pinned_at": nil}).Error
}

// IncrLikeCount 动态点赞数 +1。
func (r *Repository) IncrLikeCount(ctx context.Context, dynamicID uint) error {
	return gormx.DB(ctx, r.db).Model(&po.Dynamic{}).
		Where("id = ?", dynamicID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}

// DecrLikeCount 动态点赞数 -1（用 GREATEST 兜底不低于 0）。
func (r *Repository) DecrLikeCount(ctx context.Context, dynamicID uint) error {
	return gormx.DB(ctx, r.db).Model(&po.Dynamic{}).
		Where("id = ?", dynamicID).
		UpdateColumn("like_count", gorm.Expr("GREATEST(like_count - 1, 0)")).Error
}

// LikeCount 读取动态当前点赞数。
func (r *Repository) LikeCount(ctx context.Context, dynamicID uint) (int64, error) {
	var likeCount int64
	if err := gormx.DB(ctx, r.db).
		Model(&po.Dynamic{}).
		Select("like_count").
		Where("id = ?", dynamicID).
		Scan(&likeCount).Error; err != nil {
		return 0, err
	}

	return likeCount, nil
}

// LikeRepository 动态点赞记录仓储的 GORM 实现。
type LikeRepository struct {
	db *gorm.DB
}

var _ domain.LikeRepository = (*LikeRepository)(nil)

// NewLikeRepository 构造动态点赞仓储。
func NewLikeRepository(db *gorm.DB) *LikeRepository {
	return &LikeRepository{db: db}
}

// Create 写入点赞记录；唯一键冲突时返回 domain.ErrLikeDuplicated。
func (r *LikeRepository) Create(ctx context.Context, dynamicID, userID uint) error {
	like := po.DynamicLike{DynamicID: dynamicID, UserID: userID}
	if err := gormx.DB(ctx, r.db).Create(&like).Error; err != nil {
		if isDynamicLikeDuplicateKey(err) {
			return domain.ErrLikeDuplicated
		}

		return err
	}

	return nil
}

// Delete 删除点赞记录，返回受影响行数。
func (r *LikeRepository) Delete(ctx context.Context, dynamicID, userID uint) (int64, error) {
	res := gormx.DB(ctx, r.db).
		Where("dynamic_id = ? AND user_id = ?", dynamicID, userID).
		Delete(&po.DynamicLike{})
	if res.Error != nil {
		return 0, res.Error
	}

	return res.RowsAffected, nil
}

// isDynamicLikeDuplicateKey 判定点赞插入是否命中唯一键冲突。
//
// 判定范围与原 isDynamicLikeDuplicateKey 一致：只认 MySQL 1062，
// 不做 gorm.ErrDuplicatedKey 与错误文案匹配。
func isDynamicLikeDuplicateKey(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == mysqlDuplicateEntryErrNo
	}

	return false
}

// ReadRepository 已读位点仓储的 GORM 实现。
type ReadRepository struct {
	db *gorm.DB
}

var _ domain.ReadRepository = (*ReadRepository)(nil)

// NewReadRepository 构造已读位点仓储。
func NewReadRepository(db *gorm.DB) *ReadRepository {
	return &ReadRepository{db: db}
}

// FindByUserAndAuthor 读取用户对某作者的已读位点；不存在时返回 (nil, nil)。
func (r *ReadRepository) FindByUserAndAuthor(ctx context.Context, userID, authorID uint) (*domain.ReadRecord, error) {
	var row po.DynamicRead
	if err := gormx.DB(ctx, r.db).
		Where("user_id = ? AND author_id = ?", userID, authorID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainReadRecord(&row), nil
}

// Create 写入一条新的已读位点，并回填自增 ID。
func (r *ReadRepository) Create(ctx context.Context, record *domain.ReadRecord) error {
	if record == nil {
		return nil
	}

	row := toPOReadRecord(record)
	if err := gormx.DB(ctx, r.db).Create(row).Error; err != nil {
		return err
	}

	record.ID = row.ID

	return nil
}

// Advance 把已存在的已读位点推进到指定作品。
func (r *ReadRepository) Advance(
	ctx context.Context,
	id uint,
	workType enum.WorkType,
	workID uint,
	workAt time.Time,
) error {
	return gormx.DB(ctx, r.db).Model(&po.DynamicRead{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_read_work_type": workType,
			"last_read_work_id":   workID,
			"last_read_work_at":   workAt,
		}).Error
}
