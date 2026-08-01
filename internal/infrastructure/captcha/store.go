// Package captcha 提供验证码基础设施：点击/滑块验证码生成与校验、图形验证码存储。
package captcha

import (
	"context"
	"errors"
	"time"

	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ErrCaptchaExpired 验证码已过期或不存在（文案与原实现一致）。
var ErrCaptchaExpired = errors.New("验证码已过期或不存在")

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
		s.logger.Error("无效的captchaToken,可能已过期或不存在", zap.Error(err))

		return nil, ErrCaptchaExpired
	}

	return data, nil
}

func ensureContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil { // 没有传入上下文时使用 TODO 占位并附加短超时
		ctx = context.TODO()
	}

	return context.WithTimeout(ctx, ctxutil.ExternalShortTimeout) // 基于上游上下文叠加超时
}
