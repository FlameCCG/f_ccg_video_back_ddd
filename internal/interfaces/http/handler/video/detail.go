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

// VideoDetailReq 指定作品详情 ID，访问者身份独立从令牌解析，不从查询参数信任。
type VideoDetailReq struct {
	VideoID uint `form:"videoId" binding:"required"`
}

// VideoDetailWithProgress 将应用详情读模型与本次访问者的播放进度组合，进度不能写入共享详情缓存。
type VideoDetailWithProgress struct {
	*app.VideoDetail
	WatchProgress int `json:"watchProgress"`
}

// DetailView 以可选登录身份查询公开详情或本人预览，再补充播放进度和动态已读打点。
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
