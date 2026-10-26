package dynamic

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"time"
)

// WorkRef 作品引用（作品 ID + 发布时间），ID 为 0 表示不存在。
type WorkRef struct {
	ID        uint
	CreatedAt time.Time
}

// LatestWork 作者的「最新作品」（视频与动态综合取最新一条）。
type LatestWork struct {
	WorkType  enum.WorkType
	WorkID    uint
	CreatedAt time.Time
}

// PickLatestWork 在作者的最新视频与最新动态之间选出「最新作品」。
//
// 规则与原 getLatestWork 一致：两者都不存在时 ok 为 false；
// 动态时间**严格晚于**视频时间才算动态，时间相等时算视频。
func PickLatestWork(video, dynamic WorkRef) (LatestWork, bool) {
	if video.ID == 0 && dynamic.ID == 0 {
		return LatestWork{}, false
	}

	if dynamic.CreatedAt.After(video.CreatedAt) {
		return LatestWork{
			WorkType:  enum.WorkTypeDynamic,
			WorkID:    dynamic.ID,
			CreatedAt: dynamic.CreatedAt,
		}, true
	}

	return LatestWork{
		WorkType:  enum.WorkTypeVideo,
		WorkID:    video.ID,
		CreatedAt: video.CreatedAt,
	}, true
}

// FollowActivity 关注对象的作品活跃度快照，用于判定「最新作品」与「是否未读」。
//
// 三个时间分别来自：作者最新已发布视频时间、作者最新动态时间、
// 当前用户对该作者的已读位点时间。零值表示该维度无数据。
type FollowActivity struct {
	LatestVideoAt   time.Time
	LatestDynamicAt time.Time
	LastReadAt      time.Time
}

// Latest 返回最新作品时间与类型；视频与动态都缺失时 ok 为 false。
//
// 规则与原 ListFollowUsers / unreadFollowCountUncached 一致：
// 动态时间严格晚于视频时间时算动态，否则（含相等、含仅有视频）算视频。
func (a FollowActivity) Latest() (at time.Time, workType enum.WorkType, ok bool) {
	if a.LatestVideoAt.IsZero() && a.LatestDynamicAt.IsZero() {
		return time.Time{}, 0, false
	}

	if a.LatestDynamicAt.After(a.LatestVideoAt) {
		return a.LatestDynamicAt, enum.WorkTypeDynamic, true
	}

	return a.LatestVideoAt, enum.WorkTypeVideo, true
}

// IsUnread 判定关注对象是否有未读作品：
// 有最新作品，且已读位点缺失或早于该作品发布时间。
func (a FollowActivity) IsUnread() bool {
	at, _, ok := a.Latest()
	if !ok {
		return false
	}

	return a.LastReadAt.IsZero() || a.LastReadAt.Before(at)
}
