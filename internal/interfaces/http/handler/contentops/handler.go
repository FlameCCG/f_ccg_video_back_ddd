// Package contentops 提供轮播图、站点配置与统计、第三方登录授权 URL 的 HTTP 处理器。
//
// 处理器解析请求、装配参数并映射响应；业务逻辑由应用层处理。
// gin.Context 止于接口层，应用层仅接收 context.Context 与 DTO。
package contentops

import (
	app "f_ccg_video_back_ddd/internal/application/contentops"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"
)

// Handler 是内容运营上下文的 HTTP 处理器集合。
type Handler struct {
	banners  *app.BannerService
	stats    *app.StatService
	siteCfg  *app.SiteConfigService
	oauthURL *app.OAuthURLService
	resolver ipUtils.RegionResolver
}

// NewHandler 构造内容运营处理器，resolver 用于解析客户端 IP 归属地。
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
