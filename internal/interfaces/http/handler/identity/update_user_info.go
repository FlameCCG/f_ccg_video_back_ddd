package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// UpdateUserInfoReq 请求/响应结构体.
type UpdateUserInfoReq struct {
	Username    string `json:"username" label:"用户名"`
	Avatar      string `json:"avatar" label:"头像"`
	Description string `json:"description" label:"个人简介"`
	Gender      *int   `json:"gender" label:"性别"`
	Birthday    string `json:"birthday" label:"生日"`
}

// UpdateUserInfoView 更新用户信息.
func (h *Handler) UpdateUserInfoView(c *gin.Context) {
	req := middlewares.GetBound[UpdateUserInfoReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.profiles.UpdateProfile(c.Request.Context(), app.UpdateProfileInput{
		UserID:      uid,
		Username:    req.Username,
		Avatar:      req.Avatar,
		Description: req.Description,
		Gender:      req.Gender,
		Birthday:    req.Birthday,
	}); err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithMsg("更新成功", c)
}
