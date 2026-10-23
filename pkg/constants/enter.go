// Package constants 集中 Redis key、频道、角色名和搜索索引名等外部标识。
//
// 这些常量被生产者、消费者和存量数据共同引用，修改字符串需要考虑迁移与滚动部署兼容。
// 常量只统一命名，不定义失效、限流或授权策略；行为仍在对应上下文中。
package constants

// Redis key、集合和频道名称。它们是缓存/回写/广播双方共同遵守的协议，前缀变更需评估存量数据。
const (
	EmailCaptchaRedisKeyPrefix        = "captcha:email:"
	EmailCaptchaSendRateKeyPrefix     = "captcha:email:send:rate:"
	EmailCaptchaVerifyRateKeyPrefix   = "captcha:email:verify:rate:"
	ClickCaptchaRedisKeyPrefix        = "captcha:click:"
	SlideCaptchaRedisKeyPrefix        = "captcha:slide:"
	LoginRateKeyPrefix                = "login:rate:"
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
	// TranscodeProgressKeyPrefix 是管理端可读取的转码进度快照前缀。
	// 完整键为 transcode:progress:{videoId} 或 transcode:progress:{videoId}:{partId}。
	TranscodeProgressKeyPrefix = "transcode:progress:"
	// TranscodeProgressChannel 广播进度变更信号，订阅丢失时可重新读取进度快照。
	TranscodeProgressChannel = "transcode:progress:events"
	// TranscodeCancelChannel 向转码实例广播物理删除后的取消信号，不是持久化任务队列。
	TranscodeCancelChannel = "transcode:cancel:events"
)

// 默认角色显示名称，账号初始化与角色绑定需使用一致取值。
const (
	CommonUserRole = "普通用户"
	AdminUserRole  = "超级管理员"
	VipUserRole    = "大会员"
)

// Elasticsearch 关键词搜索索引名；向量索引使用 AI 配置中的独立索引设置。
const (
	VideoSearchIndex = "search_videos"
	UserSearchIndex  = "search_users"
)
