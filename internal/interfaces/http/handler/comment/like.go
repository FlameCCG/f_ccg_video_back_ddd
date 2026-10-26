package comment

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// LikeReq 指定点赞切换的评论 ID，点赞人取自认证上下文。
type LikeReq struct {
	CommentID uint `json:"commentId" binding:"required" label:"评论ID"`
}

// LikeResp 返回切换后的点赞数与当前用户是否已点赞，两者用于页面同步状态。
type LikeResp struct {
	LikeCount int64 `json:"likeCount"`
	IsLiked   bool  `json:"isLiked"`
}

// LikeView 调用评论点赞切换用例，返回当前计数和用户标记。
func (h *Handler) LikeView(c *gin.Context) {
	req := middlewares.GetBound[LikeReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	likeCount, isLiked, err := h.svc.ToggleLike(c.Request.Context(), uid, req.CommentID)
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithData(LikeResp{
		LikeCount: likeCount,
		IsLiked:   isLiked,
	}, c)
}
