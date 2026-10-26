package ai

import (
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// OptionsView 返回前台可用的 AI 模型与思考配置（不含密钥）.
// GET /common/ai/options.
func (h *Handler) OptionsView(c *gin.Context) {
	opts, err := h.service.PublicOptions()
	if err != nil {
		res.RecordFailure(apperr.Wrap("handler/ai/options", nil, err), c)
		res.FailWithCause(err, "AI 配置暂不可用", c)

		return
	}

	res.OkWithData(opts, c)
}
