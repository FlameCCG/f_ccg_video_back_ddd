package social

import (
	"github.com/gin-gonic/gin"
)

// FocusUserInfo 关注列表条目响应.
type FocusUserInfo = FansUserInfo

// FocusListView 获取关注列表（分页）.
// Query: id.
func (h *Handler) FocusListView(c *gin.Context) {
	relationList(c, h.relations.FocusList)
}
