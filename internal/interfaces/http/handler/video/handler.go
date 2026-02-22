// Package video 提供视频 HTTP 处理器。
//
// gin.Context 止于接口层，应用层仅接收 context.Context 与 DTO。
package video

import (
	"strconv"
	"strings"

	app "f_ccg_video_back_ddd/internal/application/video"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	wsdanmu "f_ccg_video_back_ddd/internal/interfaces/ws/danmu"
	jwts "f_ccg_video_back_ddd/pkg/jwt"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 视频公开侧与创作者侧 HTTP 处理器
// （首页流 / 详情 / 互动 / 弹幕 / 历史 / 收藏夹 / 投稿 / 上传 / 举报）。
type Handler struct {
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
	search      app.VideoSearcher
	dynamics    app.DynamicReadMarker
	hub         *wsdanmu.Hub
	cfgProvider *infraconfig.Provider
	resolver    ipUtils.RegionResolver
	logger      *zap.Logger
}

// NewHandler 构造视频 HTTP 处理器。
//
// cfgProvider 提供 Authorization 与 query token 解析所需配置，resolver 解析客户端归属地。
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
	search app.VideoSearcher,
	dynamics app.DynamicReadMarker,
	hub *wsdanmu.Hub,
	cfgProvider *infraconfig.Provider,
	resolver ipUtils.RegionResolver,
	logger *zap.Logger,
) *Handler {
	return &Handler{
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

// resolveUID 从上下文或 Header 解析用户 ID，并将成功解析的值写回 gin.Context。
// 未登录时推荐用例可回退到热门列表。
func (h *Handler) resolveUID(c *gin.Context) uint {
	if uid, ok := getUID(c); ok {
		return uid
	}

	token := c.GetHeader("Authorization")
	if token == "" {
		return 0
	}

	token = strings.TrimPrefix(token, "Bearer ")

	uidStr, _, err := jwts.ParseJwtToken(token, h.accessSecret())
	if err != nil {
		return 0
	}

	c.Set("uid", uidStr)

	uidUint64, err := strconv.ParseUint(uidStr, 10, 32)
	if err != nil {
		return 0
	}

	return uint(uidUint64)
}

// parseUIDFromToken 从 query token 或 Authorization 头解析 uid（WebSocket 入口使用）。
func (h *Handler) parseUIDFromToken(queryToken, headerToken string) uint {
	token := strings.TrimSpace(queryToken)
	if token == "" {
		token = strings.TrimSpace(headerToken)
	}

	token = strings.TrimPrefix(token, "Bearer ")
	if token == "" {
		return 0
	}

	uidStr, _, err := jwts.ParseJwtToken(token, h.accessSecret())
	if err != nil {
		return 0
	}

	uid, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil {
		return 0
	}

	return uint(uid)
}
