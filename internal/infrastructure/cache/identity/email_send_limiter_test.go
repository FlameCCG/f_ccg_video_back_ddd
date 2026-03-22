package identity

import (
	"context"
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/cache/ratelimit"
	"f_ccg_video_back_ddd/pkg/constants"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"testing"
	"time"
)

func TestEmailSendLimiterConsumesExistingKeysAndPreservesDimensions(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	limiter := NewEmailSendLimiter(ratelimit.New(client))
	for _, tc := range []struct {
		subject app.SendLimitSubject
		suffix  string
	}{
		{app.SendLimitSubject{Type: 2, Scope: app.SendLimitEmail, Value: " A@Example.COM "}, "email:2:a@example.com"},
		{app.SendLimitSubject{Type: 3, Scope: app.SendLimitIP, Value: " 2001:db8::1 "}, "ip:3:2001:db8::1"},
	} {
		key := constants.EmailCaptchaSendRateKeyPrefix + tc.suffix
		if err := client.Set(ctx, key, 1, time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
		allowed, err := limiter.Allow(ctx, tc.subject, 1, time.Minute)
		if err != nil || allowed {
			t.Fatal("legacy counter bypassed", allowed, err)
		}
		if value, err := client.Get(ctx, key).Int(); err != nil || value != 2 {
			t.Fatal(value, err)
		}
	}
	if allowed, err := NewEmailSendLimiter(nil).Allow(ctx, app.SendLimitSubject{Type: 1, Scope: app.SendLimitEmail, Value: "a@example.com"}, 1, time.Minute); err != nil || !allowed {
		t.Fatal("missing Redis fallback changed", err)
	}
}
