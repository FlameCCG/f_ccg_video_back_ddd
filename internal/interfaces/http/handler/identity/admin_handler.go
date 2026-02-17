package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"
)

// AdminHandler 提供 /v1/admin/user 管理端 HTTP 接口。
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

// NewAdminHandler 构造管理端处理器，resolver 用于解析客户端 IP 归属地。
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
