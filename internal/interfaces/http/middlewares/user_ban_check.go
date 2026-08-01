package middlewares

import (
	"errors"
	"time"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// UserBanCheck 禁言/封禁校验（用于发视频/评论/弹幕等内容操作）.
func (m *Middlewares) UserBanCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := GetUID(c)
		if !ok {
			res.FailWithMsg("请先登录", c)
			c.Abort()

			return
		}

		status, err := m.userStatus.CachedStatus(c.Request.Context(), uid)
		if err != nil {
			if errors.Is(err, identitydomain.ErrUserRecordNotFound) {
				res.FailWithMsg("用户不存在", c)
			} else {
				res.FailWithMsg("查询用户失败", c)
			}

			c.Abort()

			return
		}

		switch status {
		case enum.UserStatusActive:
			c.Next()
			return
		case enum.UserStatusPermanentlyBanned:
			res.FailWithMsg("账号已被永久封禁", c)
			c.Abort()

			return
		case enum.UserStatusBanned:
			// 允许到期自动解封
			record, err := m.banRecords.Latest(c.Request.Context(), uid)
			if err == nil && record != nil && record.Status == enum.UserStatusBanned && record.EndAt != nil {
				now := time.Now()
				if record.EndAt.Before(now) {
					unbanned, unbanErr := m.bans.ReleaseExpired(c.Request.Context(), uid, now)
					if unbanErr != nil {
						res.FailWithMsg("自动解封失败", c)
						c.Abort()

						return
					}

					if unbanned {
						c.Next()

						return
					}
				}

				msg := "账号已被封禁"
				if record.Reason != "" {
					msg = msg + "，原因：" + record.Reason
				}

				res.FailWithMsg(msg, c)
				c.Abort()

				return
			}

			res.FailWithMsg("账号已被封禁", c)
			c.Abort()

			return
		default:
			res.FailWithMsg("账号状态异常", c)
			c.Abort()

			return
		}
	}
}
