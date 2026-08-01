package identity

import (
	"strconv"

	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// RecentLikedView 返回最近点赞的10条视频（受 openLikeVideo 隐私开关约束）.
func (h *Handler) RecentLikedView(c *gin.Context) {
	targetID, ok := parseRecentActionTargetID(c)
	if !ok {
		return
	}

	viewerID, _ := middlewares.GetUID(c)

	list, total, err := h.activity.RecentLiked(c.Request.Context(), targetID, viewerID)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithList(list, total, c)
}

// RecentCoinedView 返回最近投币的10条视频（受 openCoinVideo 隐私开关约束）.
func (h *Handler) RecentCoinedView(c *gin.Context) {
	targetID, ok := parseRecentActionTargetID(c)
	if !ok {
		return
	}

	viewerID, _ := middlewares.GetUID(c)

	list, total, err := h.activity.RecentCoined(c.Request.Context(), targetID, viewerID)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithList(list, total, c)
}

// parseRecentActionTargetID 解析 userID query 参数；非法时已写响应并返回 false。
func parseRecentActionTargetID(c *gin.Context) (uint, bool) {
	target, err := strconv.Atoi(c.Query("userID"))
	if err != nil || target <= 0 {
		res.FailWithMsg("用户ID参数无效", c)

		return 0, false
	}

	return uint(target), true
}
