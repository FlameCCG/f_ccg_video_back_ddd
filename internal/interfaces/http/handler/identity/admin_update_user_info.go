package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminUpdateUserInfoReq 请求/响应结构体.
type AdminUpdateUserInfoReq struct {
	UserID      uint   `json:"userId" binding:"required,gt=0" label:"用户ID"`
	Username    string `json:"username" label:"用户名"`
	Avatar      string `json:"avatar" label:"头像"`
	Description string `json:"description" label:"个人简介"`
	Gender      *int   `json:"gender" label:"性别"`
	Birthday    string `json:"birthday" label:"生日"`
}

// AdminUpdateUserInfoView 管理员更新用户信息.
func (h *AdminHandler) AdminUpdateUserInfoView(c *gin.Context) {
	req := middlewares.GetBound[AdminUpdateUserInfoReq](c)

	// 原实现在此读取一次操作者 uid 但不使用；此处不再读取，行为不变。
	if err := h.profiles.UpdateProfile(c.Request.Context(), app.UpdateProfileInput{
		UserID:      req.UserID,
		Username:    req.Username,
		Avatar:      req.Avatar,
		Description: req.Description,
		Gender:      req.Gender,
		Birthday:    req.Birthday,
	}); err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithMsg("更新成功", c)
}
