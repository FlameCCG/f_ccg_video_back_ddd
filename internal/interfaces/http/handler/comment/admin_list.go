package comment

import (
	app "f_ccg_video_back_ddd/internal/application/comment"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// AdminCommentListReq is an exported API type.
type AdminCommentListReq struct {
	UserID   uint   `form:"userId" binding:"omitempty,gt=0"`
	Username string `form:"username"`
	Keyword  string `form:"keyword"`
	Sort     string `form:"sort" binding:"omitempty,oneof=latest oldest likes replies"`
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"pageSize" binding:"omitempty,min=1,max=100"`
}

// AdminListView 管理员评论列表.
func (h *Handler) AdminListView(c *gin.Context) {
	req := middlewares.GetBound[AdminCommentListReq](c)
	ctx := c.Request.Context()

	list, total, err := h.svc.AdminList(ctx, app.AdminListParams{
		UserID:   req.UserID,
		Username: req.Username,
		Keyword:  req.Keyword,
		Sort:     req.Sort,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithList(list, total, c)
}
