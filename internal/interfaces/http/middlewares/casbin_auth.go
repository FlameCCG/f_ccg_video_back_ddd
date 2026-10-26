package middlewares

import (
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/casbins"
	"f_ccg_video_back_ddd/pkg/res"
	"strings"

	"github.com/gin-gonic/gin"
)

// CasbinAuth Casbin 权限验证中间件.
//
// 需要在认证已写入 uid 后执行，以 user:<id>、路由模板和 HTTP 方法组成授权三元组。
// 角色菜单只影响导航展示，不能替代此处对实际 API 请求的授权。
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
			res.FailWithMsg("无权限访问", c)
			c.Abort()

			return
		}

		ok, err := m.enforcePath(sub, obj, act)
		if err != nil {
			res.RecordFailure(apperr.Wrap("middlewares/casbin_auth", nil, err), c)
			res.FailWithCause(err, "权限验证失败", c)
			c.Abort()

			return
		}

		if !ok {
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
