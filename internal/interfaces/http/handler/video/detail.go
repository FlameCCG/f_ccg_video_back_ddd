package video

import (
	"strings"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	jwts "f_ccg_video_back_ddd/pkg/jwt"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// VideoDetailReq is an exported API type.
type VideoDetailReq struct {
	VideoID uint `form:"videoId" binding:"required"`
}

// VideoDetailWithProgress is an exported API type.
type VideoDetailWithProgress struct {
	*app.VideoDetail
	WatchProgress int `json:"watchProgress"`
}

// DetailView handles the corresponding HTTP request.
func (h *Handler) DetailView(c *gin.Context) {
	req := middlewares.GetBound[VideoDetailReq](c)
	ctx := c.Request.Context()

	token := c.GetHeader("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")

	uidStr, _, err := jwts.ParseJwtToken(token, h.accessSecret())
	if err != nil {
		h.log().Debug("parse jwt token failed", zap.Error(err))
	}

	c.Set("uid", uidStr)

	var viewerID uint
	if uid, ok := getUID(c); ok {
		viewerID = uid
	}

	detail, err := h.detail.GetVideoDetail(ctx, req.VideoID, viewerID)
	if err != nil {
		if isRecordNotFound(err) {
			res.FailWithMsg("视频不存在或未发布", c)

			return
		}

		h.log().Error("get video detail failed", zap.Error(err), zap.Uint("videoId", req.VideoID))
		res.FailWithMsg("获取视频详情失败", c)

		return
	}

	// 拼上播放进度
	progress := 0

	if viewerID > 0 {
		_ = h.dynamics.MaybeMarkReadByVideo(ctx, viewerID, req.VideoID)

		if hist, histErr := h.history.GetPlayHistory(ctx, viewerID, req.VideoID); histErr == nil {
			progress = hist.Progress
		}
	}

	res.OkWithData(VideoDetailWithProgress{
		VideoDetail:   detail,
		WatchProgress: progress,
	}, c)
}
