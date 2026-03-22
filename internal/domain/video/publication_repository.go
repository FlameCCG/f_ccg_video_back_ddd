package video

import (
	"context"
)

// PublicationRepository 按业务能力声明的持久化契约。
type PublicationRepository interface {
	// Persist 在单事务内落库一次投稿，返回带持久化身份的视频与分 P，不修改输入草稿。
	Persist(ctx context.Context, draft *PublishDraft) (*PublicationResult, error)
}

// PublicationResult 是投稿持久化结果，分 P 与输入顺序一致，供提交后转码投递使用。
type PublicationResult struct {
	Video *Video
	Parts []VideoPart
}
