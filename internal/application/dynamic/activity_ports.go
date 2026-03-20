package dynamic

import (
	"context"
	"time"

	dynamicvo "f_ccg_video_back_ddd/internal/domain/dynamic/valueobject"
)

// ActivityQuery 作品活跃度与已读位点查询端口（读侧）。
type ActivityQuery interface {
	// LatestVideoTimes 批量读取作者最新已发布视频时间（key 为作者用户名）。
	LatestVideoTimes(ctx context.Context, authorUsernames []string) (map[string]time.Time, error)
	// LatestDynamicTimes 批量读取作者最新动态时间（key 为作者用户 ID）。
	LatestDynamicTimes(ctx context.Context, authorIDs []uint) (map[uint]time.Time, error)
	// LastReadTimes 批量读取当前用户对各作者的已读位点时间。
	LastReadTimes(ctx context.Context, userID uint, authorIDs []uint) (map[uint]time.Time, error)
	// LatestVideoWork 读取作者最新一条已发布视频；不存在时返回零值 WorkRef。
	LatestVideoWork(ctx context.Context, authorUsername string) (dynamicvo.WorkRef, error)
	// LatestDynamicWork 读取作者最新一条动态；不存在时返回零值 WorkRef。
	LatestDynamicWork(ctx context.Context, authorID uint) (dynamicvo.WorkRef, error)
	// PublishedVideoAuthor 读取已发布视频及其作者引用；视频不存在或未发布时返回零值。
	PublishedVideoAuthor(ctx context.Context, videoID uint) (VideoAuthorRef, error)
}
