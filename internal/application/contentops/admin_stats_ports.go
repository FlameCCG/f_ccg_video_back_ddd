package contentops

import (
	"context"
	"time"
)

// AdminStatsQuery 管理端站点统计的读侧投影端口。
//
// 统计涉及 users / videos / comments / user_login_records 四张表，
// 分属 identity、video、comment 上下文；纯 COUNT 读侧投影允许在本上下文的 infrastructure 中直读，避免为计数引入跨上下文调用链。
type AdminStatsQuery interface {
	// CountUsers 统计用户总数。
	CountUsers(ctx context.Context) (int64, error)
	// CountPublishedVideos 统计已发布视频总数。
	CountPublishedVideos(ctx context.Context) (int64, error)
	// CountComments 统计评论总数。
	CountComments(ctx context.Context) (int64, error)
	// CountUsersCreatedBetween 统计 [start, end) 区间内注册的用户数。
	CountUsersCreatedBetween(ctx context.Context, start, end time.Time) (int64, error)
	// CountDistinctLoginUsersBetween 统计 [start, end) 区间内去重的登录用户数。
	CountDistinctLoginUsersBetween(ctx context.Context, start, end time.Time) (int64, error)
	// CountPublishedVideosCreatedBetween 统计 [start, end) 区间内创建的已发布视频数。
	CountPublishedVideosCreatedBetween(ctx context.Context, start, end time.Time) (int64, error)
}
