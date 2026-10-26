package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminSetUserBannerReq 请求/响应结构体.
type AdminSetUserBannerReq struct {
	BannerID uint `json:"bannerId" binding:"required,gt=0" label:"用户主页横幅ID"`
}

// AdminSetUserBannerView 处理对应 HTTP 请求.
func (h *AdminHandler) AdminSetUserBannerView(c *gin.Context) {
	req := middlewares.GetBound[AdminSetUserBannerReq](c)

	if err := h.banners.AdminSetDefault(c.Request.Context(), req.BannerID); err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithMsg("设置成功", c)
}
