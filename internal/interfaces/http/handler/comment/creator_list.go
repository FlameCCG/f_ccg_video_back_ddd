package comment

import (
	app "f_ccg_video_back_ddd/internal/application/comment"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CreatorCommentListReq 查询当前创作者作品下的评论，作品归属通过登录身份限定。
type CreatorCommentListReq struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"pageSize" binding:"omitempty,min=1,max=50"`
	Sort     int    `form:"sort" binding:"omitempty,oneof=0 1 2"` // 0最近 1点赞最多 2回复最多
	Keyword  string `form:"keyword" binding:"omitempty,max=100"`  // 按评论内容/评论用户/视频标题搜索
}

// CreatorListView 创作中心：视频评论列表（时间倒序）.
func (h *Handler) CreatorListView(c *gin.Context) {
	req := middlewares.GetBound[CreatorCommentListReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	// 与原实现一致的二次兜底校验（GetUID 已保证 uid > 0）。
	if uid == 0 {
		res.FailWithMsg("未登录", c)

		return
	}

	list, total, err := h.svc.ListByAuthor(c.Request.Context(), app.CreatorListParams{
		AuthorID: uid,
		Page:     req.Page,
		PageSize: req.PageSize,
		Sort:     req.Sort,
		Keyword:  req.Keyword,
	})
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithList(list, total, c)
}
