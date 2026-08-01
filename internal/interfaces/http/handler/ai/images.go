package ai

import (
	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// GenerateImageView 文生图.
func (h *Handler) GenerateImageView(c *gin.Context) {
	req := bound[aiapp.ImageGenerationRequest](c)

	data, err := h.service.GenerateImage(c.Request.Context(), req)
	if err != nil {
		h.failWithAIError(c, "generate AI image failed", err)

		return
	}

	res.OkWithData(rewriteAIAssetURLs(data), c)
}

// EditImageView 图生图/图片编辑.
func (h *Handler) EditImageView(c *gin.Context) {
	req := bound[aiapp.ImageEditRequest](c)

	data, err := h.service.EditImage(c.Request.Context(), req)
	if err != nil {
		h.failWithAIError(c, "edit AI image failed", err)

		return
	}

	res.OkWithData(rewriteAIAssetURLs(data), c)
}
