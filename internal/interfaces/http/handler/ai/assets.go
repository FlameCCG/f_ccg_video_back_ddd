package ai

import (
	"errors"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/res"
	"io"

	"github.com/gin-gonic/gin"
)

const (
	eventTypeField = "type"
)

// AssetProxyReq 资源代理请求参数.
type AssetProxyReq struct {
	URL string `form:"url" binding:"required" label:"资源地址"`
}

// AssetProxyView 代理拉取上游 AI 资源（图片/视频），透传状态码与响应头.
func (h *Handler) AssetProxyView(c *gin.Context) {
	req := bound[AssetProxyReq](c)

	resp, err := h.service.OpenAssetStream(c.Request.Context(), req.URL, c.Request.Header)
	if err != nil {
		h.failWithAIError(c, "open AI asset stream failed", err)

		return
	}

	defer func() { _ = resp.Body.Close() }()

	copyUpstreamHeaders(c.Writer.Header(), resp.Header)
	c.Status(resp.StatusCode)

	if _, err = io.Copy(c.Writer, resp.Body); err != nil && !errors.Is(err, io.EOF) {
		res.RecordFailure(apperr.Wrap("handler/ai/assets", nil, err), c)
		return
	}

	res.SetSuccess(c, resp.StatusCode >= 200 && resp.StatusCode < 300)
}
