package video

import (
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	"github.com/gin-gonic/gin"
)

// getUID 从 gin.Context 安全读取当前登录用户 ID（原 api/video_api/helpers.go 的 extractUID）。
func getUID(c *gin.Context) (uint, bool) {
	return middlewares.GetUID(c)
}

// isRecordNotFound 判断错误是否为「记录不存在」。
//
// 原实现直接判 gorm.ErrRecordNotFound；新架构由领域层提供哨兵错误，
// 接口层因此无需依赖 GORM。
func isRecordNotFound(err error) bool {
	return errors.Is(err, domain.ErrRecordNotFound)
}
