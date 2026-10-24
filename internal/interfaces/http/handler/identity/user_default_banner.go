package identity

import (
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"

	app "f_ccg_video_back_ddd/internal/application/identity"
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DefaultUserBannerListRes 请求/响应结构体.
type DefaultUserBannerListRes struct {
	// BannerIDs 是系统配置中的默认横幅ID顺序。
	BannerIDs             []uint               `json:"bannerIds"`
	CurrentBannerID       uint                 `json:"currentBannerId"`
	CurrentBannerImageURL string               `json:"currentBannerImageUrl"`
	List                  []app.UserBannerItem `json:"list"`
}

// AdminUpdateUserBannerListReq 管理员更新系统默认用户横幅列表请求。
type AdminUpdateUserBannerListReq struct {
	BannerIDs []uint `json:"bannerIds" binding:"required,min=1,dive,gt=0" label:"用户主页横幅ID列表"`
	Action    int    `json:"action" binding:"required,oneof=1 2" label:"操作类型"`
}

// adminUpdateUserBannerListRes 管理员增删系统默认横幅列表的响应体。
type adminUpdateUserBannerListRes struct {
	BannerIDs []uint `json:"bannerIds"`
}

// ListDefaultUserBannersView 获取系统默认用户主页横幅列表。
func (h *Handler) ListDefaultUserBannersView(c *gin.Context) {
	// 未登录（JwtOptional 未写入 uid）时当前选择返回零值，与原实现一致。
	viewerID, _ := middlewares.GetUID(c)

	list, err := h.banners.ListDefaults(c.Request.Context(), viewerID)
	if err != nil {
		res.FailWithMsg("获取失败", c)

		return
	}

	res.OkWithData(DefaultUserBannerListRes{
		BannerIDs:             list.BannerIDs,
		CurrentBannerID:       list.CurrentBannerID,
		CurrentBannerImageURL: list.CurrentBannerImageURL,
		List:                  list.List,
	}, c)
}

// AdminUpdateUserBannerListView 管理员增删系统默认用户主页横幅列表。
func (h *AdminHandler) AdminUpdateUserBannerListView(c *gin.Context) {
	req := middlewares.GetBound[AdminUpdateUserBannerListReq](c)

	result, err := h.banners.AdminUpdateDefaults(c.Request.Context(), req.BannerIDs, req.Action)
	if err != nil {
		// 领域错误文案是「默认注册横幅不允许操作」，而本接口的原始响应文案更具体，此处显式映射。
		if errors.Is(err, domain.ErrRegisterDefaultBannerLocked) {
			res.FailWithMsg("默认注册横幅不允许在默认列表中操作", c)

			return
		}

		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(adminUpdateUserBannerListRes{
		BannerIDs: result.BannerIDs,
	}, c)
}
