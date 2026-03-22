package video

import (
	"context"
)

// PublicationRepository 按业务能力声明的持久化契约。
type PublicationRepository interface {
	// Persist 在单事务内落库一次投稿（视频 + 标签 + 分P + 初始资源），返回带主键和时间的视频。
	Persist(ctx context.Context, draft *PublishDraft) (*Video, error)
}
