package video

import (
	"context"

	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// AuthorRenameRepository 视频侧作者用户名级联更新的 GORM 实现。
//
// 绑定 identity 上下文的 VideoAuthorRenamer 端口（原 api/user_api/update_user_info.go
// 与 admin_update_user_info.go 在同一事务内的三条级联更新）。
// 经 gormx.DB(ctx, root) 取库，因此在 identity 的 tx.Manager 事务 context 内调用时
// 会复用同一事务（见 decisions.md D-010）。
type AuthorRenameRepository struct {
	db *gorm.DB
}

// NewAuthorRenameRepository 构造作者用户名级联更新仓储。
func NewAuthorRenameRepository(db *gorm.DB) *AuthorRenameRepository {
	return &AuthorRenameRepository{db: db}
}

// RenameAuthor 级联更新 videos.author_username 与两张举报表的 reporter_username。
func (r *AuthorRenameRepository) RenameAuthor(ctx context.Context, oldUsername, newUsername string) error {
	db := gormx.DB(ctx, r.db)

	if err := db.Model(&po.Video{}).
		Where("author_username = ?", oldUsername).
		Update("author_username", newUsername).Error; err != nil {
		return err
	}

	if err := db.Model(&po.VideoReport{}).
		Where("reporter_username = ?", oldUsername).
		Update("reporter_username", newUsername).Error; err != nil {
		return err
	}

	return db.Model(&po.DanmuReport{}).
		Where("reporter_username = ?", oldUsername).
		Update("reporter_username", newUsername).Error
}

// IDsByAuthor 列出某作者名下的全部视频 ID（供缓存批量失效使用）。
func (r *AuthorRenameRepository) IDsByAuthor(ctx context.Context, authorUsername string) ([]uint, error) {
	var videoIDs []uint
	if err := gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("author_username = ?", authorUsername).
		Pluck("id", &videoIDs).Error; err != nil {
		return nil, err
	}

	return videoIDs, nil
}
