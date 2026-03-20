package comment

import (
	"context"
)

// VideoCommentCounter 视频评论数增减端口（跨上下文：video 写侧）。
//
// 更新视频统计缓存并累加创作中心评论指标，
// 失败只记录日志、不返回错误。
type VideoCommentCounter interface {
	UpdateCommentCount(ctx context.Context, videoID uint, delta int64)
}

// VideoHotScoreRecorder 视频热度打点端口（跨上下文：video 写侧）。
//
// 按评论权重更新热度榜 ZSet。
type VideoHotScoreRecorder interface {
	RecordHotScoreForComment(ctx context.Context, videoID uint, delta int64)
}
