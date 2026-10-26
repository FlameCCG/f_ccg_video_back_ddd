package video

import (
	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DetailReq 指定作品详情 ID，访问者身份独立从令牌解析，不从查询参数信任。
type DetailReq struct {
	VideoID uint `form:"videoId" binding:"required"`
}

// DetailWithProgress 将应用详情读模型与本次访问者的播放进度组合，进度不能写入共享详情缓存。
type DetailWithProgress struct {
	*app.Detail
	WatchProgress int `json:"watchProgress"`
}

// DetailView 以可选登录身份查询公开详情或本人预览，再补充播放进度和动态已读打点。
func (h *Handler) DetailView(c *gin.Context) {
	req := middlewares.GetBound[DetailReq](c)
	ctx := c.Request.Context()

	viewerID := h.resolveUID(c)

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

	res.OkWithData(DetailWithProgress{
		Detail:        detail,
		WatchProgress: progress,
	}, c)
}
