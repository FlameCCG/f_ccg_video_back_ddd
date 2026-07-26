package ai

import (
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// OptionsView 返回前台可用的 AI 模型与思考配置（不含密钥）.
// GET /common/ai/options
func (h *Handler) OptionsView(c *gin.Context) {
	opts, err := h.service.PublicOptions()
	if err != nil {
		zap.L().Error("get AI public options failed", zap.Error(err))
		res.FailWithMsg("AI 配置暂不可用", c)

		return
	}

	res.OkWithData(opts, c)
}
