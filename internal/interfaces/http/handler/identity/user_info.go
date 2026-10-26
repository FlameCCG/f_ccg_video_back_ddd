package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// UserInfoRes 请求/响应结构体.
type UserInfoRes struct {
	IsAdmin        bool     `json:"isAdmin"`
	ID             uint     `json:"id"`
	Username       string   `json:"username"`
	Email          string   `json:"email"`
	Avatar         string   `json:"avatar"`
	BannerURL      string   `json:"bannerUrl"`
	Description    string   `json:"description"`
	Gender         int      `json:"gender"`
	Birthday       string   `json:"birthday"`
	Level          int      `json:"level"`
	Exp            int64    `json:"exp"`
	CoinCount      float64  `json:"coinCount"`
	FollowCount    int64    `json:"followCount"`
	FansCount      int64    `json:"fansCount"`
	DynamicCount   int64    `json:"dynamicCount"`
	RegisterSource string   `json:"registerSource"`
	LikeTags       []string `json:"likeTags"`
	// HasPassword 是否已设置登录密码；第三方首次登录用户可能为 false。
	// 前端「修改密码」可据此决定是否要求输入旧密码。
	HasPassword bool `json:"hasPassword"`
	// IsThirdParty 是否绑定了第三方登录（OpenID 非空）。
	IsThirdParty bool `json:"isThirdParty"`
}

// UserInfoView 获取当前登录用户信息.
func (h *Handler) UserInfoView(c *gin.Context) {
	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	info, err := h.profiles.UserInfo(c.Request.Context(), uid)
	if err != nil {
		res.FailWithCause(err, "用户不存在", c)

		return
	}

	res.OkWithData(UserInfoRes{
		IsAdmin:        info.IsAdmin,
		ID:             info.ID,
		Username:       info.Username,
		Email:          info.Email,
		Avatar:         info.Avatar,
		BannerURL:      info.BannerURL,
		Description:    info.Description,
		Gender:         info.Gender,
		Birthday:       info.Birthday,
		Level:          info.Level,
		Exp:            info.Exp,
		CoinCount:      info.CoinCount,
		FollowCount:    info.FollowCount,
		FansCount:      info.FansCount,
		DynamicCount:   info.DynamicCount,
		RegisterSource: info.RegisterSource,
		LikeTags:       info.LikeTags,
		HasPassword:    info.HasPassword,
		IsThirdParty:   info.IsThirdParty,
	}, c)
}
