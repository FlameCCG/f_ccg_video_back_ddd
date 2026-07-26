package social

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/social"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// FocusRepository 关注关系仓储的 GORM 实现。
type FocusRepository struct {
	db *gorm.DB
}

var _ domain.FocusRepository = (*FocusRepository)(nil)

// NewFocusRepository 构造关注关系仓储。
func NewFocusRepository(db *gorm.DB) *FocusRepository {
	return &FocusRepository{db: db}
}

// Create 写入关注关系；唯一键冲突时返回 domain.ErrAlreadyFocused。
//
// 冲突判定复用 gormx.IsDuplicateKeyError，判定范围与原 isUserFocusDuplicate
// 完全一致（gorm.ErrDuplicatedKey / MySQL 1062 / 错误文案匹配）。
func (r *FocusRepository) Create(ctx context.Context, focus *domain.Focus) error {
	if focus == nil {
		return nil
	}

	row := toPOFocus(focus)
	if err := gormx.DB(ctx, r.db).Create(row).Error; err != nil {
		if gormx.IsDuplicateKeyError(err) {
			return domain.ErrAlreadyFocused
		}

		return err
	}

	if created := toDomainFocus(row); created != nil {
		*focus = *created
	}

	return nil
}

// Delete 删除关注关系，返回受影响行数。
func (r *FocusRepository) Delete(ctx context.Context, userID, focusUserID uint) (int64, error) {
	res := gormx.DB(ctx, r.db).
		Where("user_id = ? AND focus_user_id = ?", userID, focusUserID).
		Delete(&po.UserFocus{})
	if res.Error != nil {
		return 0, res.Error
	}

	return res.RowsAffected, nil
}
