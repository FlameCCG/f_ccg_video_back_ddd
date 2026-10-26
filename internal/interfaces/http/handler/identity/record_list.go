package identity

import (
	"context"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// RecordListReq carries pagination shared by the authenticated account ledgers.
type RecordListReq struct {
	Page     int `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=100" label:"每页数量"`
}

func serveUserRecords[T any](c *gin.Context, req RecordListReq, list func(context.Context, uint, int, int) ([]T, int64, error)) {
	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	rows, total, err := list(c.Request.Context(), uid, req.Page, req.PageSize)
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)
		return
	}

	res.OkWithList(rows, total, c)
}
