package chat

import (
	"f_ccg_video_back_ddd/pkg/res"
	"strconv"

	"github.com/gin-gonic/gin"
)

// WSView 私信 WebSocket 入口（支持 Authorization header 或 query token 传递 token）.
func (h *Handler) WSView(c *gin.Context) {
	uidStr := c.GetString("uid")
	if uidStr == "" {
		res.FailWithMsg(testLoginRequired, c)

		return
	}

	uid, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil || uid == 0 {
		res.FailWithMsg(testLoginRequired, c)

		return
	}

	if err := h.hub.ServeWS(c, uint(uid)); err != nil {
		res.RecordFailure(err, c)

		return
	}
}
