package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminUserInfoRes 请求/响应结构体.
type AdminUserInfoRes struct {
	ID             uint            `json:"id"`
	Username       string          `json:"username"`
	Email          string          `json:"email"`
	Avatar         string          `json:"avatar"`
	Description    string          `json:"description"`
	Gender         int             `json:"gender"`
	Birthday       string          `json:"birthday"`
	Level          int             `json:"level"`
	Exp            int64           `json:"exp"`
	CoinCount      float64         `json:"coinCount"`
	FollowCount    int64           `json:"followCount"`
	FansCount      int64           `json:"fansCount"`
	DynamicCount   int64           `json:"dynamicCount"`
	CurrentRoleID  uint            `json:"currentRoleID"`
	RegisterSource string          `json:"registerSource"`
	RoleMap        map[uint]string `json:"roleMap"`
}

// AdminUserInfoView 获取后台当前登录用户信息.
func (h *AdminHandler) AdminUserInfoView(c *gin.Context) {
	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	info, err := h.profiles.AdminUserInfo(c.Request.Context(), uid)
	if err != nil {
		res.FailWithMsg("用户不存在", c)

		return
	}

	res.OkWithData(AdminUserInfoRes{
		ID:             info.ID,
		Username:       info.Username,
		Email:          info.Email,
		Avatar:         info.Avatar,
		Description:    info.Description,
		Gender:         info.Gender,
		Birthday:       info.Birthday,
		Level:          info.Level,
		Exp:            info.Exp,
		CoinCount:      info.CoinCount,
		FollowCount:    info.FollowCount,
		FansCount:      info.FansCount,
		DynamicCount:   info.DynamicCount,
		CurrentRoleID:  info.CurrentRoleID,
		RegisterSource: info.RegisterSource,
		RoleMap:        info.RoleMap,
	}, c)
}
