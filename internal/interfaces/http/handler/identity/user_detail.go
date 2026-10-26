package identity

import (
	"f_ccg_video_back_ddd/pkg/res"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// UserDetailRes 请求/响应结构体.
type UserDetailRes struct {
	ID          uint     `json:"id"`
	Username    string   `json:"username"`
	Avatar      string   `json:"avatar"`
	BannerURL   string   `json:"bannerUrl"`
	Description string   `json:"description"`
	Gender      int      `json:"gender"`
	Birthday    string   `json:"birthday"`
	Level       int      `json:"level"`
	Exp         int64    `json:"exp"`
	FollowCount int64    `json:"followCount"`
	FansCount   int64    `json:"fansCount"`
	TotalLikes  int64    `json:"totalLikes"`
	TotalViews  int64    `json:"totalViews"`
	LikeTags    []string `json:"likeTags"`
}

// UserDetailView 根据用户ID获取用户详情（公开信息）.
// Query: id.
func (h *Handler) UserDetailView(c *gin.Context) {
	userID := strings.TrimSpace(c.Query("id"))
	if userID == "" {
		res.FailWithMsg("用户ID不能为空", c)

		return
	}

	uid, err := strconv.Atoi(userID)
	if err != nil {
		res.FailWithCause(err, "用户ID格式错误", c)

		return
	}

	detail, err := h.profiles.UserDetail(c.Request.Context(), uint(uid))
	if err != nil {
		res.FailWithCause(err, "用户不存在", c)

		return
	}

	res.OkWithData(UserDetailRes{
		ID:          detail.ID,
		Username:    detail.Username,
		Avatar:      detail.Avatar,
		BannerURL:   detail.BannerURL,
		Description: detail.Description,
		Gender:      detail.Gender,
		Birthday:    detail.Birthday,
		Level:       detail.Level,
		Exp:         detail.Exp,
		FollowCount: detail.FollowCount,
		FansCount:   detail.FansCount,
		TotalLikes:  detail.TotalLikes,
		TotalViews:  detail.TotalViews,
		LikeTags:    detail.LikeTags,
	}, c)
}
