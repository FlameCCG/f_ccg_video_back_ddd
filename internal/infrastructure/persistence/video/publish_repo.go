package video

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// Persist 在单事务内落库一次投稿（视频 + 标签 + 分P + 初始资源）。
//
// 事务内的错误文案与原 api/video_api/publish.go 的 persistVideo 逐字一致；
// 源文件名超长等 DB 错误经 domain.MapSourceFileNameDBError 转为可读业务文案。
func (r *VideoRepository) Persist(ctx context.Context, draft *domain.PublishDraft) error {
	if draft == nil || draft.Video == nil {
		return domain.ErrParamInvalid
	}

	row := toPOVideo(draft.Video)

	err := gormx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if createErr := tx.Create(row).Error; createErr != nil {
			return domain.ErrCreateVideoFailed
		}
		// nil 也落库，明确标记新投稿没有定时计划，避免误查旧 Redis。
		if err := tx.Create(&po.VideoPublishSchedule{VideoID: row.ID, PublishAt: draft.PublishAt}).Error; err != nil {
			return err
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
				return domain.ErrCreateVideoTagsFailed
			}
		}

		if len(draft.Parts) > 0 {
			parts := make([]po.VideoPart, 0, len(draft.Parts))
			for _, part := range draft.Parts {
				parts = append(parts, toPOVideoPart(part, row.ID))
			}

			if createErr := tx.Create(&parts).Error; createErr != nil {
				return domain.ErrCreatePartsFailed
			}

			// 回填自增 ID 供调用方派发转码任务
			for i := range parts {
				if i < len(draft.Parts) {
					draft.Parts[i].ID = parts[i].ID
					draft.Parts[i].VideoID = row.ID
				}
			}

			if resErr := createInitialPartResources(tx, draft, parts); resErr != nil {
				if domain.IsSourceFileNameError(resErr) {
					return resErr
				}

				return domain.ErrCreatePartResourcesFailed
			}

			return nil
		}

		if resErr := createInitialVideoResource(tx, draft, row.ID); resErr != nil {
			if domain.IsSourceFileNameError(resErr) {
				return resErr
			}

			return domain.ErrCreateVideoResourceFailed
		}

		return nil
	})
	if err != nil {
		return err
	}

	draft.Video.ID = row.ID
	draft.Video.CreatedAt = row.CreatedAt
	draft.Video.UpdatedAt = row.UpdatedAt

	return nil
}

// createInitialPartResources 逐个分P写入初始（源片）资源行。
func createInitialPartResources(tx *gorm.DB, draft *domain.PublishDraft, parts []po.VideoPart) error {
	for idx := range parts {
		if idx >= len(draft.PartResources) {
			continue
		}

		resource := toPOVideoPartResource(draft.PartResources[idx], parts[idx].ID)
		if resource == nil {
			continue
		}

		if err := tx.Create(&resource).Error; err != nil {
			return domain.MapSourceFileNameDBError(err, resource.SourceFileName)
		}
	}

	return nil
}

// createInitialVideoResource 为单视频作品写入初始（源片）资源行。
func createInitialVideoResource(tx *gorm.DB, draft *domain.PublishDraft, videoID uint) error {
	resource := toPOVideoResource(draft.VideoResource, videoID)
	if resource == nil {
		return nil
	}

	if err := tx.Create(resource).Error; err != nil {
		return domain.MapSourceFileNameDBError(err, resource.SourceFileName)
	}

	return nil
}
