package middlewares

import (
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/res"
	"time"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"

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
				res.FailWithCause(err, "用户不存在", c)
			} else {
				res.FailWithCause(err, "查询用户失败", c)
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
			m.checkTemporaryBan(c, uid)
			return
		default:
			res.FailWithMsg("账号状态异常", c)
			c.Abort()

			return
		}
	}
}

func middlewaresUserBanCheckCondition(record *identitydomain.BanRecord, err error) bool {
	return err == nil && record != nil && record.Status == enum.UserStatusBanned && record.EndAt != nil
}

func (m *Middlewares) checkTemporaryBan(c *gin.Context, uid uint) {
	record, err := m.banRecords.Latest(c.Request.Context(), uid)
	if err != nil {
		res.FailWithCause(err, "账号已被封禁", c)
		c.Abort()

		return
	}

	if !middlewaresUserBanCheckCondition(record, err) {
		res.FailWithMsg("账号已被封禁", c)
		c.Abort()

		return
	}

	now := time.Now()
	if record.EndAt.Before(now) {
		unbanned, err := m.bans.ReleaseExpired(c.Request.Context(), uid, now)
		if err != nil {
			res.FailWithCause(err, "自动解封失败", c)
			c.Abort()

			return
		}

		if unbanned {
			c.Next()
			return
		}
	}

	message := "账号已被封禁"
	if record.Reason != "" {
		message += "，原因：" + record.Reason
	}

	res.FailWithMsg(message, c)
	c.Abort()
}
