package video

import (
	"context"

	videodomain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// Persist 在单事务中写入视频、标签、分 P 与初始资源。
//
// publish_at 与 is_scheduled 同步写入视频行；事务不回滚已上传对象，也不保证提交后的 Kafka 投递。
// 源文件名超长等数据库错误经 mapSourceFileNameDBError 转为用户可读文案。
func (r *VideoRepository) Persist(ctx context.Context, draft *videodomain.PublishDraft) (*videodomain.Video, error) {
	if draft == nil || draft.Video == nil {
		return nil, videodomain.ErrParamInvalid
	}

	row := toPOVideo(draft.Video)
	row.PublishAt = draft.PublishAt
	row.IsScheduled = draft.PublishAt != nil

	err := gormx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if createErr := tx.Create(row).Error; createErr != nil {
			return videodomain.ErrCreateVideoFailed
		}
		if len(draft.TagIDs) > 0 {
			tags := make([]po.Tag, 0, len(draft.TagIDs))
			for _, id := range draft.TagIDs {
				tags = append(tags, po.Tag{Model: po.Model{ID: id}})
			}

			values := make([]any, 0, len(tags))
			for i := range tags {
				values = append(values, &tags[i])
			}

			if assocErr := tx.Model(row).Association("Tags").Replace(values...); assocErr != nil {
				return videodomain.ErrCreateVideoTagsFailed
			}
		}

		if len(draft.Parts) > 0 {
			parts := make([]po.VideoPart, 0, len(draft.Parts))
			for _, part := range draft.Parts {
				parts = append(parts, toPOVideoPart(part, row.ID))
			}

			if createErr := tx.Create(&parts).Error; createErr != nil {
				return videodomain.ErrCreatePartsFailed
			}

			// 回填自增 ID 供调用方派发转码任务
			for i := range parts {
				if i < len(draft.Parts) {
					draft.Parts[i].ID = parts[i].ID
					draft.Parts[i].VideoID = row.ID
				}
			}

			if resErr := createInitialPartResources(tx, draft, parts); resErr != nil {
				if videodomain.IsSourceFileNameError(resErr) {
					return resErr
				}

				return videodomain.ErrCreatePartResourcesFailed
			}

			return nil
		}

		if resErr := createInitialVideoResource(tx, draft, row.ID); resErr != nil {
			if videodomain.IsSourceFileNameError(resErr) {
				return resErr
			}

			return videodomain.ErrCreateVideoResourceFailed
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 事务成功后用已落库的主键、时间和定时标记重建实体，不改写调用方传入的旧指针。
	snap := draft.Video.Snapshot()
	snap.ID = row.ID
	snap.CreatedAt = row.CreatedAt
	snap.UpdatedAt = row.UpdatedAt
	snap.IsScheduled = row.IsScheduled
	restored := videodomain.RestoreVideo(snap)
	draft.Video = restored

	return restored, nil
}

// createInitialPartResources 逐个分P写入初始（源片）资源行。
func createInitialPartResources(tx *gorm.DB, draft *videodomain.PublishDraft, parts []po.VideoPart) error {
	for idx := range parts {
		if idx >= len(draft.PartResources) {
			continue
		}

		resource := toPOVideoPartResource(draft.PartResources[idx], parts[idx].ID)
		if resource == nil {
			continue
		}

		if err := tx.Create(&resource).Error; err != nil {
			return mapSourceFileNameDBError(err, resource.SourceFileName)
		}
	}

	return nil
}

// createInitialVideoResource 为单视频作品写入初始（源片）资源行。
func createInitialVideoResource(tx *gorm.DB, draft *videodomain.PublishDraft, videoID uint) error {
	resource := toPOVideoResource(draft.VideoResource, videoID)
	if resource == nil {
		return nil
	}

	if err := tx.Create(resource).Error; err != nil {
		return mapSourceFileNameDBError(err, resource.SourceFileName)
	}

	return nil
}
