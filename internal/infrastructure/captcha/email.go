package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"

	"github.com/redis/go-redis/v9"
)

// EmailCaptchaStore 邮箱验证码存储的 Redis 实现。
//
// key 前缀与载荷 JSON 结构与原实现完全一致
// （原逻辑分散在 api/captcha_api/email_captcha.go 与
// pkg/middlewares/email_captcha_verify.go）。
type EmailCaptchaStore struct {
	redis *redis.Client
}

var _ app.EmailCaptchaStore = (*EmailCaptchaStore)(nil)

// NewEmailCaptchaStore 构造邮箱验证码存储适配器。
func NewEmailCaptchaStore(redisClient *redis.Client) *EmailCaptchaStore {
	return &EmailCaptchaStore{redis: redisClient}
}

// Store 写入验证码载荷并设置过期时间。
func (s *EmailCaptchaStore) Store(ctx context.Context, emailID, email, code string, ttl time.Duration) error {
	timeout, cancel := ctxutil.WithTimeout(ctx, ctxutil.AsyncDefaultTimeout)
	defer cancel()

	val, err := json.Marshal(map[string]string{
		"code":  code,
		"email": email,
	})
	if err != nil {
		return err
	}

	return s.redis.Set(timeout, constants.EmailCaptchaRedisKeyPrefix+emailID, val, ttl).Err()
}

// Consume 原子 GETDEL 取出验证码载荷。
func (s *EmailCaptchaStore) Consume(ctx context.Context, emailID string) (*app.EmailCaptchaPayload, error) {
	data, err := s.redis.GetDel(ctx, constants.EmailCaptchaRedisKeyPrefix+emailID).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, apperr.Wrap("captcha.email.consume", ErrCaptchaExpired, err, apperr.Rejected)
		}

		return nil, apperr.Wrap("captcha.email.consume", nil, err, apperr.Dependency)
	}

	var payload app.EmailCaptchaPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}
