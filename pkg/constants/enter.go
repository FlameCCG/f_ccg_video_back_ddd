// Package constants provides package functionality.
package constants

// Redis key captcha prefixes.
const (
	EmailCaptchaRedisKeyPrefix        = "captcha:email:"
	EmailCaptchaSendRateKeyPrefix     = "captcha:email:send:rate:"
	EmailCaptchaVerifyRateKeyPrefix   = "captcha:email:verify:rate:"
	ClickCaptchaRedisKeyPrefix        = "captcha:click:"
	SlideCaptchaRedisKeyPrefix        = "captcha:slide:"
	VideoScheduleKey                  = "video:schedule"
	VideoDetailCacheKeyPrefix         = "video:detail:meta:"
	VideoDetailMetaGenKeyPrefix       = "video:detail:meta:gen:"
	VideoDetailStatsKeyPrefix         = "video:detail:stats:"
	VideoDetailStatsDirtySetKey       = "video:detail:stats:dirty"
	VideoDetailStatsVersionKeyPrefix  = "video:detail:stats:ver:"
	CommentLikeUserSetPrefix          = "comment:like:users:"
	CommentLikeCountKeyPrefix         = "comment:like:count:"
	CommentLikeQueueKey               = "comment:like:queue"
	CommentLikeProcessingQueueKey     = "comment:like:queue:processing"
	UserStatusCacheKeyPrefix          = "user:status:"
	UserStatusGenKeyPrefix            = "user:status:gen:"
	BannerVisibleCacheKeyPrefix       = "banner:visible:"
	PartitionSubmittableCacheKey      = "partition:submittable"
	FolderListCacheKeyPrefix          = "folder:list:"
	NotificationCountCacheKeyPrefix   = "notification:count:"
	DynamicUnreadFollowCacheKeyPrefix = "dynamic:unread:follow:"
	DanmuChannelPrefix                = "danmu:channel:"
	DanmuRecentKeyPrefix              = "danmu:recent:"
	DanmuOnlineZSetPrefix             = "danmu:online:"
	ChatUserChannelPrefix             = "chat:user:"
	VideoHotRankZSetKey               = "video:hot:zset"
	SearchHotKeywordZSetKey           = "search:hot:keywords:zset"
	// TranscodeProgressKeyPrefix Redis key prefix for admin-visible transcode progress.
	// Full key: transcode:progress:{videoId} or transcode:progress:{videoId}:{partId}
	TranscodeProgressKeyPrefix = "transcode:progress:"
	// TranscodeProgressChannel carries best-effort progress change notifications.
	TranscodeProgressChannel = "transcode:progress:events"
	// TranscodeCancelChannel broadcasts hard-delete cancellation events to every transcode instance.
	TranscodeCancelChannel = "transcode:cancel:events"
)

// default role.
const (
	CommonUserRole = "普通用户"
	AdminUserRole  = "超级管理员"
	VipUserRole    = "大会员"
)

// Elasticsearch index names.
const (
	VideoSearchIndex = "search_videos"
	UserSearchIndex  = "search_users"
)
