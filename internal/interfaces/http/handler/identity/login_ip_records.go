package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"

	"github.com/gin-gonic/gin"
)

// LoginIPRecordListReq 请求/响应结构体.
type LoginIPRecordListReq = RecordListReq

// LoginIPRecordListView 获取当前用户登录IP记录（需登录）.
func (h *Handler) LoginIPRecordListView(c *gin.Context) {
	req := middlewares.GetBound[LoginIPRecordListReq](c)
	serveUserRecords(c, req, h.loginRecords.List)
}
