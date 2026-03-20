package repository

import (
	"context"

	videoentity "f_ccg_video_back_ddd/internal/domain/video/entity"
)

// PublicationRepository 按业务能力声明的持久化契约。
type PublicationRepository interface {
	// Persist 在单事务内落库一次投稿（视频 + 标签 + 分P + 初始资源）。
	Persist(ctx context.Context, draft *videoentity.PublishDraft) error
}
