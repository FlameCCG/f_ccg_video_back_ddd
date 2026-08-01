package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"
)

// AdminHandler 管理端（/v1/admin/user）HTTP 处理器。
//
// 对应原 api/user_api.UserAPI 中挂在 admin 分组下的方法。
type AdminHandler struct {
	logins   *app.LoginService
	users    *app.AdminUserService
	accounts *app.AccountService
	profiles *app.ProfileService
	banners  *app.UserBannerService
	bans     *app.BanService
	records  *app.BanRecordService
	resolver ipUtils.RegionResolver
}

// NewAdminHandler 构造管理端处理器。
//
// resolver 用于把客户端 IP 解析为归属地（原 ipUtils 直读 global.IPSearcher，见 decisions.md D-005）。
func NewAdminHandler(
	logins *app.LoginService,
	users *app.AdminUserService,
	accounts *app.AccountService,
	profiles *app.ProfileService,
	banners *app.UserBannerService,
	bans *app.BanService,
	records *app.BanRecordService,
	resolver ipUtils.RegionResolver,
) *AdminHandler {
	return &AdminHandler{
		logins:   logins,
		users:    users,
		accounts: accounts,
		profiles: profiles,
		banners:  banners,
		bans:     bans,
		records:  records,
		resolver: resolver,
	}
}
