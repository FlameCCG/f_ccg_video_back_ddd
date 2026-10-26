// Package video 把上传、投稿、改稿、播放、互动、弹幕和管理的 HTTP 请求翻译为应用用例调用。
//
// handler.go 声明显式依赖，其他文件按入口拆分；参数结构体保留 Gin 绑定与校验标签。
// 从请求取得参数和经过鉴权的身份，构造应用参数后传 c.Request.Context()，最后映射错误和响应。
// gin.Context 止步于接口层，领域规则、事务、缓存与跨上下文调用由 application 编排。
// publish.go/upload.go 将上传与投稿命令交给应用服务；admin_* 入口的管理权限来自路由中间件。
// JSON 字段、状态码和响应文案属于对外契约，新增注释不应顺带调整这些行为。
package video

import (
	"context"
	"errors"
	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/pkg/authsession"
	"strconv"
	"strings"

	app "f_ccg_video_back_ddd/internal/application/video"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	wsdanmu "f_ccg_video_back_ddd/internal/interfaces/ws/danmu"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 视频公开侧与创作者侧 HTTP 处理器
// （首页流 / 详情 / 互动 / 弹幕 / 历史 / 收藏夹 / 投稿 / 上传 / 举报）。
type Handler struct {
	userStatus  *identityapp.UserStatusService
	feed        *app.FeedService
	detail      *app.DetailService
	recommend   *app.RecommendService
	interact    *app.InteractionService
	danmu       *app.DanmuService
	history     *app.HistoryService
	folders     *app.FolderService
	partitions  *app.PartitionService
	publish     *app.PublishService
	update      *app.UpdateService
	uploads     *app.UploadService
	reports     *app.ReportService
	search      app.Searcher
	dynamics    app.DynamicReadMarker
	hub         *wsdanmu.Hub
	cfgProvider *infraconfig.Provider
	resolver    ipUtils.RegionResolver
	logger      *zap.Logger
}

// NewHandler 构造视频 HTTP 处理器。
//
// cfgProvider 用于解析 Authorization / query token 中的 uid（原实现直读 global.Config()）；
// resolver 供举报图片等路径解析客户端归属地。
func NewHandler(
	feed *app.FeedService,
	detail *app.DetailService,
	recommend *app.RecommendService,
	interact *app.InteractionService,
	danmu *app.DanmuService,
	history *app.HistoryService,
	folders *app.FolderService,
	partitions *app.PartitionService,
	publish *app.PublishService,
	update *app.UpdateService,
	uploads *app.UploadService,
	reports *app.ReportService,
	search app.Searcher,
	dynamics app.DynamicReadMarker,
	hub *wsdanmu.Hub,
	cfgProvider *infraconfig.Provider,
	resolver ipUtils.RegionResolver,
	logger *zap.Logger,
	userStatus *identityapp.UserStatusService,
) *Handler {
	return &Handler{
		userStatus:  userStatus,
		feed:        feed,
		detail:      detail,
		recommend:   recommend,
		interact:    interact,
		danmu:       danmu,
		history:     history,
		folders:     folders,
		partitions:  partitions,
		publish:     publish,
		update:      update,
		uploads:     uploads,
		reports:     reports,
		search:      search,
		dynamics:    dynamics,
		hub:         hub,
		cfgProvider: cfgProvider,
		resolver:    resolver,
		logger:      loggerOrNop(logger),
	}
}

func (h *Handler) log() *zap.Logger {
	if h == nil {
		return zap.NewNop()
	}

	return loggerOrNop(h.logger)
}

func loggerOrNop(logger *zap.Logger) *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}

	return logger
}

// accessSecret 读取当前生效的 JWT access 密钥（支持配置热更新）。
func (h *Handler) accessSecret() string {
	if h.cfgProvider == nil {
		return ""
	}

	cfg := h.cfgProvider.Get()
	if cfg == nil || cfg.Jwt == nil {
		return ""
	}

	return cfg.Jwt.AccessSecret
}

// resolveUID 尝试从上下文或 Header 解析用户 ID；推荐逻辑可在未登录时回退热门。
//
// 与原 api/video_api/home.go 的 resolveUID 逐行一致（含把解析出的 uid 写回 gin.Context）。
func (h *Handler) resolveUID(c *gin.Context) uint {
	if uid, ok := getUID(c); ok {
		return uid
	}

	if h.userStatus == nil {
		return 0
	}
	token := c.GetHeader("Authorization")
	if token == "" {
		return 0
	}

	token = strings.TrimPrefix(token, "Bearer ")

	user, err := h.userStatus.ValidateAccessToken(c.Request.Context(), token, h.accessSecret())
	if err != nil || user.MustChangePassword {
		return 0
	}

	uidStr := strconv.FormatUint(uint64(user.ID), 10)
	c.Set("uid", uidStr)

	return user.ID
}

// parseUIDFromToken 从 query token 或 Authorization 头解析 uid（WebSocket 入口使用）。
func (h *Handler) parseUIDFromToken(c *gin.Context, queryToken, headerToken string) uint {
	token := strings.TrimSpace(queryToken)
	if token == "" {
		token = strings.TrimSpace(headerToken)
	}

	token = strings.TrimPrefix(token, "Bearer ")
	if token == "" {
		return 0
	}

	if h.userStatus == nil {
		return 0
	}
	user, err := h.userStatus.ValidateAccessToken(c.Request.Context(), token, h.accessSecret())
	if err != nil || user.MustChangePassword {
		return 0
	}
	c.Request = c.Request.WithContext(authsession.WithValidator(c.Request.Context(), func(ctx context.Context) error {
		current, err := h.userStatus.ValidateAccessToken(ctx, token, h.accessSecret())
		if err != nil {
			return err
		}
		if current.MustChangePassword {
			return errors.New("initial password change required")
		}
		return nil
	}))
	return user.ID
}
