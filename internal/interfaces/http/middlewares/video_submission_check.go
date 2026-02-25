package middlewares

import (
	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// VideoSubmissionCheck 实时读取站点开关，禁止普通用户调用投稿相关接口。
// 必须位于 JwtAuth 之后、参数绑定和文件读取之前；管理员仍需通过封禁校验。
func (m *Middlewares) VideoSubmissionCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := GetUID(c)
		if !ok {
			res.FailWithMsg("请先登录", c)
			c.Abort()
			return
		}
		cfg := m.cfgProvider.Get()
		if cfg == nil || cfg.Site == nil || !cfg.Site.DisableUserSubmission {
			c.Next()
			return
		}
		admin, err := identityapp.IsAdminUser(c.Request.Context(), m.userRoles, uid)
		if err != nil {
			m.log().Error("check video submission administrator failed", zap.Error(err), zap.Uint("uid", uid))
			res.FailWithMsg("投稿权限验证失败，请稍后重试", c)
			c.Abort()
			return
		}
		if !admin {
			res.FailWithMsg("网站已暂停用户投稿，仅管理员可上传、发布或修改视频", c)
			c.Abort()
			return
		}
		c.Next()
	}
}
