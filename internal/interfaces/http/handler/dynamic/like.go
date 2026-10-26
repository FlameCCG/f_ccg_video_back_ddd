package dynamic

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// LikeReq 动态点赞请求.
type LikeReq struct {
	DynamicID uint `json:"dynamicId" binding:"required,gt=0" label:"动态ID"`
}

// LikeResp 动态点赞响应.
type LikeResp struct {
	LikeCount int64 `json:"likeCount"`
	IsLiked   bool  `json:"isLiked"`
}

// LikeView 切换动态点赞状态.
func (h *Handler) LikeView(c *gin.Context) {
	req := middlewares.GetBound[LikeReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	ctx := c.Request.Context()

	result, err := h.likes.ToggleLike(ctx, uid, req.DynamicID)
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)
		return
	}

	res.OkWithData(LikeResp{
		LikeCount: result.LikeCount,
		IsLiked:   result.IsLiked,
	}, c)
}
