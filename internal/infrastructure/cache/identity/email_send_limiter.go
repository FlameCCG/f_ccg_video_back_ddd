package identity

import (
	"context"
	"fmt"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/cache/ratelimit"
	"f_ccg_video_back_ddd/pkg/constants"
)

// EmailSendLimiter 复用固定窗口原子计数，并兼容既有邮箱/IP Redis 键。
type EmailSendLimiter struct{ limiter *ratelimit.Limiter }

var _ app.RateLimiter = (*EmailSendLimiter)(nil)

func NewEmailSendLimiter(limiter *ratelimit.Limiter) *EmailSendLimiter {
	return &EmailSendLimiter{limiter: limiter}
}

func (l *EmailSendLimiter) Allow(ctx context.Context, subject app.SendLimitSubject, limit int64, window time.Duration) (bool, error) {
	return l.limiter.Allow(ctx, l.key(subject), limit, window)
}

func (*EmailSendLimiter) key(subject app.SendLimitSubject) string {
	value := strings.TrimSpace(subject.Value)
	if subject.Scope == app.SendLimitEmail {
		value = strings.ToLower(value)
	}
	return fmt.Sprintf("%s%s:%d:%s", constants.EmailCaptchaSendRateKeyPrefix, subject.Scope, subject.Type, value)
}
