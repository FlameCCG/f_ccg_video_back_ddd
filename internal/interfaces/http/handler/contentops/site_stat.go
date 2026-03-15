package contentops

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"

	"github.com/gin-gonic/gin"
)

// AdminStatsView 管理员站点统计（需管理员权限）.
func (h *Handler) AdminStatsView(c *gin.Context) {
	ctx := c.Request.Context()

	stats, err := h.stats.AdminStats(ctx)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(stats, c)
}

// StatTouchView 站点统计打点：增加流量；登录用户当日首次触发登录经验.
func (h *Handler) StatTouchView(c *gin.Context) {
	uid, _ := middlewares.GetUID(c)
	ip, _ := ipUtils.GetClientIPAndAddr(c, h.resolver)

	ctx := c.Request.Context()

	result, err := h.stats.Touch(ctx, uid, ip)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(result, c)
}

// StatHeartbeatView 在线心跳：不增加PV，仅维护UV与在线人数（支持游客）.
func (h *Handler) StatHeartbeatView(c *gin.Context) {
	uid, _ := middlewares.GetUID(c)
	ip, _ := ipUtils.GetClientIPAndAddr(c, h.resolver)

	ctx := c.Request.Context()

	result, err := h.stats.Heartbeat(ctx, uid, ip)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(result, c)
}
