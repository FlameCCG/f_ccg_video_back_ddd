// Package captcha 实现交互验证码、图形验证码和邮件验证码存储。
//
// click.go/slide.go 适配验证码库，store.go/email.go 实现状态存取，email_generator.go 生成邮件验证码。
// 生成、发送、验证和限流分属不同步骤，是否需要某类验证码由登录场景与站点配置共同决定。
//
// 具体对象由 bootstrap 构造并通过端口交给内层，技术实现不应反过来要求业务层导入本包。
package captcha

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ErrCaptchaExpired 验证码已过期或不存在（文案与原实现一致）。
var ErrCaptchaExpired = apperr.Wrap("", nil, errors.New("验证码已过期或不存在"), apperr.Rejected)

// payloadStore 封装验证码答案在 Redis 中的存取，供点击/滑块验证码复用。
type payloadStore struct {
	redis  *redis.Client
	logger *zap.Logger
}

// store 写入答案并设置过期时间。
func (s payloadStore) store(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctxWithTimeout, cancel := ensureContext(ctx) // 为写操作附加超时
	defer cancel()                               // 操作完成后释放上下文

	return s.redis.Set(ctxWithTimeout, key, value, ttl).Err() // 写入答案并设置过期时间
}

// consume 原子 GETDEL，确保验证码一次性消费。
func (s payloadStore) consume(ctx context.Context, key string) ([]byte, error) {
	ctxWithTimeout, cancel := ensureContext(ctx)
	defer cancel()

	data, err := s.redis.GetDel(ctxWithTimeout, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, apperr.Wrap("captcha.consume", ErrCaptchaExpired, err, apperr.Rejected)
		}

		return nil, apperr.Wrap("captcha.consume", ErrCaptchaExpired, err, apperr.Dependency)
	}

	return data, nil
}

func ensureContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil { // 没有传入上下文时使用 TODO 占位并附加短超时
		ctx = context.TODO()
	}

	return context.WithTimeout(ctx, ctxutil.ExternalShortTimeout) // 基于上游上下文叠加超时
}
