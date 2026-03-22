package video

// 用户视频列表排序。
const (
	// UserVideoSortViews 按播放量排序.
	UserVideoSortViews = 1
	// UserVideoSortFavorite 按收藏排序.
	UserVideoSortFavorite = 2
)

// 用户视频审核态。
const (
	// UserVideoAuditPublished 已发布审核态.
	UserVideoAuditPublished = 1
	// UserVideoAuditPrivate 私密审核态.
	UserVideoAuditPrivate = 2
	// UserVideoAuditReviewing 审核中.
	UserVideoAuditReviewing = 4
	// UserVideoAuditScheduled 定时待发布.
	UserVideoAuditScheduled = 5
)
