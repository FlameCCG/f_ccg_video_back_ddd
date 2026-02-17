package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// SwitchRoleReq 请求/响应结构体.
type SwitchRoleReq struct {
	RoleID uint `json:"roleId" binding:"required,gt=0"`
}

// SwitchRoleView 切换当前用户角色.
func (h *AdminHandler) SwitchRoleView(c *gin.Context) {
	req := middlewares.GetBound[SwitchRoleReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.accounts.SwitchRole(c.Request.Context(), uid, req.RoleID); err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithMsg("切换成功", c)
}
