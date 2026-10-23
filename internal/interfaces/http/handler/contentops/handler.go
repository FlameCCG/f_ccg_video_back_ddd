// Package contentops 把站点配置、统计、轮播图与 OAuth 授权 URL的 HTTP 请求翻译为应用用例调用。
//
// handler.go 声明显式依赖，其他文件按入口拆分；参数结构体保留 Gin 绑定与校验标签。
// 从请求取得参数和经过鉴权的身份，构造应用参数后传 c.Request.Context()，最后映射错误和响应。
// gin.Context 止步于接口层，领域规则、事务、缓存与跨上下文调用由 application 编排。
// 具体入口与权限链到 internal/interfaces/http/router 中查看，业务允许条件还需由应用与领域判断。
// JSON 字段、状态码和响应文案属于对外契约，新增注释不应顺带调整这些行为。
package contentops

import (
	app "f_ccg_video_back_ddd/internal/application/contentops"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"
)

// Handler 内容运营上下文的处理器集合。
//
// 方法名与原 api/banner_api.BannerAPI、api/site_api.SiteAPI 的导出方法完全一致。
type Handler struct {
	banners  *app.BannerService
	stats    *app.StatService
	siteCfg  *app.SiteConfigService
	oauthURL *app.OAuthURLService
	resolver ipUtils.RegionResolver
}

// NewHandler 构造内容运营处理器。
//
// resolver 用于把客户端 IP 解析为归属地（原 global.IPSearcher，见 decisions.md D-005）。
func NewHandler(
	banners *app.BannerService,
	stats *app.StatService,
	siteCfg *app.SiteConfigService,
	oauthURL *app.OAuthURLService,
	resolver ipUtils.RegionResolver,
) *Handler {
	return &Handler{
		banners:  banners,
		stats:    stats,
		siteCfg:  siteCfg,
		oauthURL: oauthURL,
		resolver: resolver,
	}
}
