package comment

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// purgeVideoIDBatchSize 按视频批量清理评论时的分批大小（与原 admin_delete_service 的分批一致）。
const purgeVideoIDBatchSize = 200

// PurgeRepository 评论级联清理仓储。
//
// 供 dynamic（删除动态）与 video（删除视频）两个上下文经端口调用：
// 评论与评论点赞的所有权归 comment 上下文，跨上下文的写侧一律走这里。
type PurgeRepository struct {
	db *gorm.DB
}

// NewPurgeRepository 构造评论级联清理仓储。
func NewPurgeRepository(db *gorm.DB) *PurgeRepository {
	return &PurgeRepository{db: db}
}

// PurgeByDynamic 删除该动态下的全部评论与评论点赞，返回各自删除行数。
//
// 与原 service/dynamic_service/dynamic_service.go 的事务内清理逻辑一致：
// 先按 dynamic_id 取评论 ID，再删点赞，最后删评论。
func (r *PurgeRepository) PurgeByDynamic(ctx context.Context, dynamicID uint) (int64, int64, error) {
	var (
		deletedComments     int64
		deletedCommentLikes int64
	)

	err := gormx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var commentIDs []uint
		if err := tx.Model(&po.Comment{}).
			Where("dynamic_id = ?", dynamicID).
			Pluck("id", &commentIDs).Error; err != nil {
			return err
		}

		if len(commentIDs) > 0 {
			likeResult := tx.Where("comment_id IN ?", commentIDs).Delete(&po.CommentLike{})
			if likeResult.Error != nil {
				return likeResult.Error
			}

			deletedCommentLikes = likeResult.RowsAffected
		}

		commentResult := tx.Where("dynamic_id = ?", dynamicID).Delete(&po.Comment{})
		if commentResult.Error != nil {
			return commentResult.Error
		}

		deletedComments = commentResult.RowsAffected

		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	return deletedComments, deletedCommentLikes, nil
}

// PurgeByVideoIDs 删除给定视频的全部评论与评论点赞，返回删除计数。
//
// 与原 service/video_service/admin_delete_service.go 一致：按 ID 分批执行，
// 避免超长 IN 列表；调用方已在事务中时经 gormx.DB 复用该事务。
func (r *PurgeRepository) PurgeByVideoIDs(ctx context.Context, videoIDs []uint) (int64, int64, error) {
	if len(videoIDs) == 0 {
		return 0, 0, nil
	}

	var (
		deletedComments     int64
		deletedCommentLikes int64
	)

	db := gormx.DB(ctx, r.db)

	for start := 0; start < len(videoIDs); start += purgeVideoIDBatchSize {
		end := min(start+purgeVideoIDBatchSize, len(videoIDs))

		batch := videoIDs[start:end]

		var commentIDs []uint
		if err := db.Model(&po.Comment{}).
			Where("video_id IN ?", batch).
			Pluck("id", &commentIDs).Error; err != nil {
			return 0, 0, err
		}

		if len(commentIDs) > 0 {
			likeResult := db.Where("comment_id IN ?", commentIDs).Delete(&po.CommentLike{})
			if likeResult.Error != nil {
				return 0, 0, likeResult.Error
			}

			deletedCommentLikes += likeResult.RowsAffected
		}

		commentResult := db.Where("video_id IN ?", batch).Delete(&po.Comment{})
		if commentResult.Error != nil {
			return 0, 0, commentResult.Error
		}

		deletedComments += commentResult.RowsAffected
	}

	return deletedComments, deletedCommentLikes, nil
}
