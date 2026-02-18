package comment

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CommentLikeReq is an exported API type.
type CommentLikeReq struct {
	CommentID uint `json:"commentId" binding:"required" label:"评论ID"`
}

// CommentLikeResp is an exported API type.
type CommentLikeResp struct {
	LikeCount int64 `json:"likeCount"`
	IsLiked   bool  `json:"isLiked"`
}

// LikeView handles the corresponding HTTP request.
func (h *Handler) LikeView(c *gin.Context) {
	req := middlewares.GetBound[CommentLikeReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	likeCount, isLiked, err := h.svc.ToggleLike(c.Request.Context(), uid, req.CommentID)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(CommentLikeResp{
		LikeCount: likeCount,
		IsLiked:   isLiked,
	}, c)
}
