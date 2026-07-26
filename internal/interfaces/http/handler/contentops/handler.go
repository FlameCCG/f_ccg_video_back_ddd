// Package contentops 是内容运营上下文的 HTTP 处理器：
// 轮播图（原 api/banner_api）与站点配置 / 站点统计 / 三方登录授权链接（原 api/site_api）。
//
// 处理器只做请求解析、参数装配与响应文案映射；业务逻辑全部下沉到 application 层。
// gin.Context 止步于本包：向应用层只传 context.Context 与 DTO。
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
