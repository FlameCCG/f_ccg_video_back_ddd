package video

import (
	"errors"

	videoerrors "f_ccg_video_back_ddd/internal/domain/video/domainerr"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"

	"github.com/gin-gonic/gin"
)

// getUID 从 gin.Context 安全读取当前登录用户 ID。
func getUID(c *gin.Context) (uint, bool) {
	return middlewares.GetUID(c)
}

// isRecordNotFound 判断错误是否为领域层的记录缺失哨兵，接口层无需依赖 GORM。
func isRecordNotFound(err error) bool {
	return errors.Is(err, videoerrors.ErrRecordNotFound)
}
