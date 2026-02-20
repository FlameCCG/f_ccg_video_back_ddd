package dynamic

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DynamicLikeReq 动态点赞请求.
type DynamicLikeReq struct {
	DynamicID uint `json:"dynamicId" binding:"required,gt=0" label:"动态ID"`
}

// DynamicLikeResp 动态点赞响应.
type DynamicLikeResp struct {
	LikeCount int64 `json:"likeCount"`
	IsLiked   bool  `json:"isLiked"`
}

// LikeView 切换动态点赞状态.
func (h *Handler) LikeView(c *gin.Context) {
	req := middlewares.GetBound[DynamicLikeReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	ctx := c.Request.Context()

	result, err := h.likes.ToggleLike(ctx, uid, req.DynamicID)
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	res.OkWithData(DynamicLikeResp{
		LikeCount: result.LikeCount,
		IsLiked:   result.IsLiked,
	}, c)
}
