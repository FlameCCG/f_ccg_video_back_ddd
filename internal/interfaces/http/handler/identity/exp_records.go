package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"

	"github.com/gin-gonic/gin"
)

// ExpRecordListReq 请求/响应结构体.
type ExpRecordListReq = RecordListReq

// ExpRecordListView 获取当前用户经验记录（需登录）.
func (h *Handler) ExpRecordListView(c *gin.Context) {
	req := middlewares.GetBound[ExpRecordListReq](c)
	serveUserRecords(c, req, h.expRecords.List)
}
