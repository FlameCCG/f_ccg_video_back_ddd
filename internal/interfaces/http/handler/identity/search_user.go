package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// UserSearchReq 请求/响应结构体.
type UserSearchReq struct {
	Keyword   string `form:"keyword" binding:"required"`
	Page      int    `form:"page" binding:"omitempty,min=1"`
	PageSize  int    `form:"pageSize" binding:"omitempty,min=1,max=100"`
	UserSort  int    `form:"userSort" binding:"omitempty,oneof=0 1 2 3"` // 0相关性(_score) 1粉丝数 2等级 3注册时间
	UserOrder int    `form:"userOrder" binding:"omitempty,oneof=0 1"`    // 0降序 1升序
}

// SearchView 处理对应 HTTP 请求.
func (h *Handler) SearchView(c *gin.Context) {
	req := middlewares.GetBound[UserSearchReq](c)

	// 分页与排序归一化在应用层完成（原 handler 内联的 normalizeUserSort / normalizeOrder）。
	data, err := h.activity.SearchUsers(c.Request.Context(), app.UserSearchParams{
		Keyword:   req.Keyword,
		Page:      req.Page,
		PageSize:  req.PageSize,
		UserSort:  req.UserSort,
		UserOrder: req.UserOrder,
	})
	if err != nil {
		res.FailWithCause(err, "搜索失败", c)

		return
	}

	res.OkWithData(data, c)
}
