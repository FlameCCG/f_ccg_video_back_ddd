package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CreatorAnalyticsReq 请求/响应结构体.
type CreatorAnalyticsReq struct {
	Range string `form:"range" binding:"omitempty,oneof=7d 30d month 7 30 week 1m" label:"时间范围"`
	Type  string `form:"type" binding:"omitempty,oneof=fans views comments coins danmu favorites" label:"指标类型"`
}

// CreatorAnalyticsView 创作中心数据分析（需登录）.
func (h *Handler) CreatorAnalyticsView(c *gin.Context) {
	req := middlewares.GetBound[CreatorAnalyticsReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	data, err := h.creators.Analytics(c.Request.Context(), uid, req.Range, req.Type)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(data, c)
}

// CreatorOverviewView 创作中心总览（需登录）.
func (h *Handler) CreatorOverviewView(c *gin.Context) {
	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	data, err := h.creators.Overview(c.Request.Context(), uid)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(data, c)
}
