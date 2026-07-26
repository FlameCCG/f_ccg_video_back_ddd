package identity

import (
	"context"
	"io"
	"time"
)

// ---------------------------------------------------------------------------
// 本上下文的技术端口（实现在 internal/infrastructure/external/* 与 cache/*）
// ---------------------------------------------------------------------------

// OAuthStateStore 一次性 OAuth state 存储端口。
//
// 实现见 internal/infrastructure/external/oauth.StateStore：Redis 优先、
// 进程内内存兜底，key 形如 oauth:state:{provider}:{state}，TTL 10 分钟。
type OAuthStateStore interface {
	// Store 保存一次性 state。
	Store(ctx context.Context, provider, state string) error
	// Consume 校验并消费 state；无效/过期/已消费时返回错误。
	Consume(ctx context.Context, provider, state string) error
}

// OAuthUserInfo 第三方用户信息（各 provider 适配器归一化后的结果）。
type OAuthUserInfo struct {
	// OpenID 第三方用户唯一标识。
	OpenID string
	// Email 第三方返回的邮箱，可能为空。
	Email string
	// FallbackEmail 邮箱缺失时用于拼装兜底邮箱的本地部分（linuxdo 传其用户名）。
	FallbackEmail string
	// Avatar 第三方头像地址。
	Avatar string
}

// QQOAuthClient QQ 授权码换用户信息端口。
type QQOAuthClient interface {
	FetchUser(ctx context.Context, code string) (*OAuthUserInfo, error)
}

// GoogleOAuthClient Google 授权码换用户信息端口。
type GoogleOAuthClient interface {
	FetchUser(ctx context.Context, code string) (*OAuthUserInfo, error)
}

// GitHubOAuthClient GitHub 授权码换用户信息端口（支持 PKCE code_verifier）。
type GitHubOAuthClient interface {
	FetchUser(ctx context.Context, code, codeVerifier string) (*OAuthUserInfo, error)
}

// LinuxDoOAuthClient Linux.do 授权码换用户信息端口。
type LinuxDoOAuthClient interface {
	FetchUser(ctx context.Context, code string) (*OAuthUserInfo, error)
}

// XOAuthClient X (Twitter) 授权码换用户信息端口（强制 PKCE）。
type XOAuthClient interface {
	FetchUser(ctx context.Context, code, codeVerifier string) (*OAuthUserInfo, error)
}

// EmailCaptchaSender 邮箱验证码投递端口。
//
// captchaType 与接口层一致：1 注册 / 2 重置密码 / 3 绑定邮箱。
// 原实现是 api/captcha_api 的包级函数变量 emailCaptchaSend（为测试打桩），
// 新架构改为注入端口，测试注入 fake。
type EmailCaptchaSender interface {
	SendEmailCaptcha(captchaType int, email, code string) error
}

// EmailVerifyResult 邮箱可达性校验结果（字段对应原 emailverifier.Result 的判定分支）。
type EmailVerifyResult struct {
	// SyntaxValid 语法是否合法。
	SyntaxValid bool
	// HasSMTPResult 是否拿到了 SMTP 探测结果（对应 ret.SMTP != nil）。
	HasSMTPResult bool
	// SMTPHostExists SMTP 主机是否存在。
	SMTPHostExists bool
	// HasMXRecords 域名是否有 MX 记录。
	HasMXRecords bool
}

// EmailAddressVerifier 邮箱地址可达性校验端口。
//
// 原实现是 api/captcha_api 的包级 emailverifier 实例，新架构改为注入端口。
type EmailAddressVerifier interface {
	Verify(email string) (*EmailVerifyResult, error)
}

// RateLimiter 固定窗口限流端口（实现为 infrastructure/cache/ratelimit.Limiter）。
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)
}

// ConfigFileWriter 配置文件写回端口。
//
// 管理端改动系统默认横幅后需要把 site 配置段持久化回 YAML（原 viper.WriteConfig）。
// 与 application/contentops.ConfigFileWriter 同签名，可绑定同一实现。
type ConfigFileWriter interface {
	// Write 把 item 写入配置文件的 name 配置段。
	Write(name string, item any) error
}

// ---------------------------------------------------------------------------
// 本上下文的读侧查询端口（实现在 internal/infrastructure/persistence/identity）
// ---------------------------------------------------------------------------

// UserBannerItem 用户主页横幅读模型（json 字段与原 mysqlEntity.Banner 逐字一致）。
type UserBannerItem struct {
	ID          uint   `json:"id"`
	Cover       string `json:"cover"`
	Href        string `json:"href"`
	Show        bool   `json:"show"`
	Type        int    `json:"type"`
	PartitionID uint   `json:"partitionId"`
}

// UserBannerCatalog 用户主页横幅目录查询端口（读侧投影，见 decisions.md D-011）。
type UserBannerCatalog interface {
	// FindVisible 读取指定 ID 的可见用户横幅；不存在时返回 (nil, nil)。
	FindVisible(ctx context.Context, bannerID uint) (*UserBannerItem, error)
	// ListVisible 按传入 ID 顺序读取可见用户横幅，缺失的跳过。
	ListVisible(ctx context.Context, bannerIDs []uint) ([]UserBannerItem, error)
}

// UserRelationCounts 关注/粉丝/动态计数（字段与原 user_service.UserRelationCounts 一致）。
type UserRelationCounts struct {
	FocusCount   int64
	FansCount    int64
	DynamicCount int64
}

// UserActionPrivacy 最近点赞/投币公开开关（字段与原 user_service.UserActionPrivacy 一致）。
type UserActionPrivacy struct {
	Exists        bool
	OpenLikeVideo bool
	OpenCoinVideo bool
}

// UserRelationQuery 关注关系与隐私开关查询端口（读侧投影，见 decisions.md D-011）。
type UserRelationQuery interface {
	// RelationCounts 一次 SQL 取关注数/粉丝数（withDynamic 为 true 时附带动态数）。
	RelationCounts(ctx context.Context, userID uint, withDynamic bool) UserRelationCounts
	// DynamicCount 单独统计用户动态数。
	DynamicCount(ctx context.Context, userID uint) int64
	// ActionPrivacy 读取最近点赞/投币是否公开。
	ActionPrivacy(ctx context.Context, userID uint) (UserActionPrivacy, error)
}

// UserRoleQuery 用户角色查询端口（读侧投影，见 decisions.md D-011）。
type UserRoleQuery interface {
	// RoleIDByName 按角色名读取角色 ID；不存在时返回错误。
	RoleIDByName(ctx context.Context, name string) (uint, error)
	// HasRole 判断用户是否拥有指定角色。
	HasRole(ctx context.Context, userID, roleID uint) (bool, error)
	// LoadRoles 加载用户角色列表（OAuth 登录链路的可用性校验，见 report.md FIND-013）。
	LoadRoles(ctx context.Context, userID uint) ([]UserRoleItem, error)
	// RoleMap 返回用户「角色 ID → 角色名」映射；无角色时返回 nil。
	RoleMap(ctx context.Context, userID uint) (map[uint]string, error)
}

// UserRoleItem 用户角色读模型。
type UserRoleItem struct {
	ID   uint
	Name string
}

// UserConfView 用户配置读模型（json 字段与原 mysqlEntity.UserConf 逐字一致）。
//
// LikeTags 保持数据库中的原始 JSON 字节，保证 GET /user/info/conf 响应逐字不变。
type UserConfView struct {
	LikeTags      []byte `json:"likeTags"`
	BannerID      uint   `json:"bannerId"`
	BannerURL     string `json:"bannerUrl"`
	OpenCollect   bool   `json:"openCollect"`
	OpenFans      bool   `json:"openFans"`
	OpenFollow    bool   `json:"openFollow"`
	OpenLikeVideo bool   `json:"openLikeVideo"`
	OpenCoinVideo bool   `json:"openCoinVideo"`
}

// UserConfQuery 用户配置读侧端口。
type UserConfQuery interface {
	// View 读取用户配置读模型；不存在时返回 (nil, nil)。
	View(ctx context.Context, userID uint) (*UserConfView, error)
}

// ---------------------------------------------------------------------------
// 跨上下文端口：由 bootstrap 绑定到对应上下文的应用服务 / 适配器
// ---------------------------------------------------------------------------

// UserRoleBinder Casbin 用户-角色绑定端口（写侧）。
//
// 跨上下文依赖：应绑定到 rbac 上下文的 Casbin 能力
// （原 global.CasbinEnforcer.AddRoleForUser(casbins.GetUserSub(uid), roleName)）。
type UserRoleBinder interface {
	// AddRoleForUser 为用户绑定角色；实现负责把 userID 转换为 Casbin subject。
	AddRoleForUser(userID uint, roleName string) error
}

// DefaultFolderEnsurer 默认收藏夹兜底创建端口。
//
// 跨上下文依赖：应绑定到 video 上下文的收藏夹服务
// （原 video_service.FolderService.EnsureDefaultFolder，登录时调用且忽略错误）。
type DefaultFolderEnsurer interface {
	EnsureDefaultFolder(ctx context.Context, userID uint) error
}

// 用户主页「最近动作」类型（取值必须与 video 上下文一致）。
const (
	// VideoActionLike 最近点赞。
	VideoActionLike = "like"
	// VideoActionCoin 最近投币。
	VideoActionCoin = "coin"
)

// UserVideoQuery 用户主页视频列表与最近动作查询端口。
//
// 跨上下文依赖：应绑定到 video 上下文（原 video_service.UserVideoService）。
// 返回的 items 会被接口层原样交给 res.OkWithList / res.OkWithData 序列化，
// 因此实现必须直接返回 video 上下文自己的读模型（原 []UserVideoItem），
// 以保证响应 JSON 与原项目逐字一致。
type UserVideoQuery interface {
	// ListUserVideos 用户主页视频分页列表。
	ListUserVideos(ctx context.Context, userID, viewerID uint, page, pageSize, sort, auditStatus int) (items any, total int64, err error)
	// RecentActions 最近点赞/投币的视频列表（action 取 VideoActionLike / VideoActionCoin）。
	RecentActions(ctx context.Context, userID uint, action string, limit int) (items any, count int, err error)
}

// UserVideoStats 作者已发布视频的曝光/互动汇总（字段与原 video_service.UserVideoStats 一致）。
type UserVideoStats struct {
	TotalLikes int64
	TotalViews int64
}

// PublishedVideoStatsQuery 作者已发布视频统计端口。
//
// 跨上下文依赖：应绑定到 video 上下文（原 video_service.AggregateUserVideoStats，
// 内部带 Redis 统计缓存 + singleflight，故不在本上下文复刻）。
type PublishedVideoStatsQuery interface {
	AggregateByAuthor(ctx context.Context, authorUsername string) UserVideoStats
}

// VideoTagWriter 标签创建端口。
//
// 跨上下文依赖：应绑定到 video 上下文（原 video_service.TagService.CreateOrGet）。
// 返回值原样交给 res.OkWithData，实现须返回 video 上下文自己的标签读模型。
type VideoTagWriter interface {
	CreateOrGetTag(ctx context.Context, name string) (any, error)
}

// VideoRemover 用户删除自己视频的端口。
//
// 跨上下文依赖：应绑定到 video 上下文（原 video_service.DeleteVideo）。
type VideoRemover interface {
	DeleteVideo(ctx context.Context, videoID, userID uint) error
}

// VideoAuthorRenamer 视频侧作者用户名级联更新端口（写侧）。
//
// 跨上下文依赖：应绑定到 video 上下文，需级联更新
// videos.author_username、video_reports.reporter_username、danmu_reports.reporter_username。
// 本端口由 identity 在 tx.Manager 的事务 context 内调用，
// 实现必须经 gormx.DB(ctx, root) 取库，才能与用户改名落在同一事务
// （等价原 update_user_info.go 中的单事务级联）。
type VideoAuthorRenamer interface {
	RenameAuthor(ctx context.Context, oldUsername, newUsername string) error
}

// VideoCacheInvalidator 视频侧缓存失效端口。
//
// 跨上下文依赖：应绑定到 video 上下文
// （原 video_service.InvalidateUserUsernameCache / InvalidateVideoMetaCache /
// InvalidateVideoAuthorCache）。
type VideoCacheInvalidator interface {
	// InvalidateUsername 失效「userID → username」派生缓存。
	InvalidateUsername(userID uint)
	// InvalidateAuthorVideos 批量失效该作者名下视频的详情缓存；
	// usernameChanged 为 true 时连同 authorID 缓存一起清。
	InvalidateAuthorVideos(ctx context.Context, authorUsername string, usernameChanged bool)
}

// 用户搜索排序字段/方向（取值必须与 search 上下文一致）。
const (
	// UserSortRelevance 按相关性（ES _score）排序。
	UserSortRelevance = 0
	// UserSortFollowers 按粉丝数排序。
	UserSortFollowers = 1
	// UserSortLevel 按等级排序。
	UserSortLevel = 2
	// UserSortCreatedAt 按注册时间排序。
	UserSortCreatedAt = 3

	// SortOrderDesc 降序。
	SortOrderDesc = 0
	// SortOrderAsc 升序。
	SortOrderAsc = 1
)

// UserSearchParams 用户搜索参数。
type UserSearchParams struct {
	Keyword   string
	Page      int
	PageSize  int
	UserSort  int
	UserOrder int
}

// UserSearchQuery 用户搜索端口。
//
// 跨上下文依赖：应绑定到 search 上下文（原 search_service.SearchService.SearchUsers）。
// 返回值原样交给 res.OkWithData，实现须返回 search 上下文自己的 *UserSearchResult。
type UserSearchQuery interface {
	SearchUsers(ctx context.Context, params UserSearchParams) (any, error)
}

// MentionSuggestQuery 评论 @ 联想端口。
//
// 跨上下文依赖：应绑定到 search 上下文（原 search_service.SearchService.SuggestFollowUsers）。
// 返回的 items 原样交给 res.OkWithList，实现须返回 search 上下文自己的 []MentionUser。
type MentionSuggestQuery interface {
	SuggestFollowUsers(ctx context.Context, userID uint, keyword string, page, pageSize int) (items any, total int64, err error)
}

// BannerStorage 用户主页横幅文件存储端口。
//
// 跨上下文依赖：应绑定到 media/storage 上下文
// （原 storage_service.ValidateUploadID + ResolveDriver().SaveFile）。
type BannerStorage interface {
	// ValidateUploadID 校验前端传入的文件哈希是否是安全的上传标识。
	ValidateUploadID(uploadID string) error
	// EnsureReady 校验存储驱动配置可用。
	// 对应原 storage_service.ResolveDriver 的错误分支（响应「存储配置错误」）。
	EnsureReady() error
	// SaveBanner 保存横幅文件并返回可访问 URL。
	SaveBanner(ctx context.Context, relativePath string, size int64, reader io.Reader, contentType string) (string, error)
}
