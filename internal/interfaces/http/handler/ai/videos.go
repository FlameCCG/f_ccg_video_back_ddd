package ai

import (
	"strings"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// GenerateVideoView 创建视频生成任务.
func (h *Handler) GenerateVideoView(c *gin.Context) {
	req := bound[aiapp.VideoGenerationRequest](c)

	data, err := h.service.GenerateVideo(c.Request.Context(), req)
	if err != nil {
		failWithAIError(c, "generate AI video failed", err)

		return
	}

	res.OkWithData(data, c)
}

// VideoStatusView 查询视频生成任务状态.
// Query: requestID
func (h *Handler) VideoStatusView(c *gin.Context) {
	requestID := strings.TrimSpace(c.Query("requestID"))
	if requestID == "" {
		res.FailWithMsg("requestID 不能为空", c)

		return
	}

	data, err := h.service.GetVideo(c.Request.Context(), requestID)
	if err != nil {
		failWithAIError(c, "get AI video status failed", err)

		return
	}

	res.OkWithData(rewriteAIAssetURLs(data), c)
}
