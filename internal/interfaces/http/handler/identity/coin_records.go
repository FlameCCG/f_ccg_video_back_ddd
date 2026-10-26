package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"

	"github.com/gin-gonic/gin"
)

// CoinRecordListReq 请求/响应结构体.
type CoinRecordListReq = RecordListReq

// CoinRecordListView 获取当前用户硬币记录（需登录）.
func (h *Handler) CoinRecordListView(c *gin.Context) {
	req := middlewares.GetBound[CoinRecordListReq](c)
	serveUserRecords(c, req, h.coinRecords.List)
}
