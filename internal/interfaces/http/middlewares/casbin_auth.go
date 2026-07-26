package middlewares

import (
	"strings"

	"f_ccg_video_back_ddd/pkg/casbins"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CasbinAuth Casbin 权限验证中间件.
//
// 路由均为静态路径，业务参数走 query/body，鉴权对象为 Gin FullPath（与 -rs 同步的资源表路径一致）。
// 匹配规则见 conf/rbac_model.conf 的 globMatch（精确路径 + * / ** 通配）。
func (m *Middlewares) CasbinAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.GetString("uid")
		act := c.Request.Method
		sub := casbins.GetUserSub(uid)
		fullPath := strings.TrimSpace(c.FullPath())
		realPath := c.Request.URL.Path

		// 优先 FullPath（路由模板，静态路由下与资源表一致）；异常时回退真实路径
		obj := fullPath
		if obj == "" {
			obj = realPath
		}
		if obj == "" {
			zap.L().Warn("Casbin 鉴权时路径为空",
				zap.String("uid", uid),
				zap.String("method", act))
			res.FailWithMsg("无权限访问", c)
			c.Abort()
			return
		}

		ok, err := m.enforcePath(sub, obj, act)
		if err != nil {
			zap.L().Error("Casbin 权限验证失败", zap.Error(err))
			res.FailWithMsg("权限验证失败", c)
			c.Abort()
			return
		}

		if !ok {
			zap.L().Warn("用户无权限访问",
				zap.String("uid", uid),
				zap.String("path", realPath),
				zap.String("fullPath", fullPath),
				zap.String("method", act))
			res.FailWithMsg("无权限访问", c)
			c.Abort()
			return
		}

		if shouldSkipNext(c) {
			return
		}

		c.Next()
	}
}

func (m *Middlewares) enforcePath(sub, obj, act string) (bool, error) {
	if m.enforcer == nil {
		return false, nil
	}

	return m.enforcer.Enforce(sub, obj, act)
}
