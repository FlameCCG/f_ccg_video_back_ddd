package captcha

import (
	"encoding/json"
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/pkg/constants"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"strings"
	"testing"
	"time"
)

func TestClickCaptchaRejectsRepeatedOrReorderedTargets(t *testing.T) {
	for _, test := range []struct {
		name   string
		points []app.ClickCaptchaPoint
		want   bool
	}{
		{"ordered distinct", []app.ClickCaptchaPoint{{Index: 0, X: 5, Y: 5}, {Index: 1, X: 105, Y: 105}}, true},
		{"duplicate first", []app.ClickCaptchaPoint{{Index: 0, X: 5, Y: 5}, {Index: 0, X: 5, Y: 5}}, false},
		{"reordered", []app.ClickCaptchaPoint{{Index: 1, X: 105, Y: 105}, {Index: 0, X: 5, Y: 5}}, false},
		{"index does not match point", []app.ClickCaptchaPoint{{Index: 0, X: 105, Y: 105}, {Index: 1, X: 5, Y: 5}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: server.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			captcha := NewClickCaptcha(client, nil, zap.NewNop())
			payload, _ := json.Marshal(clickCaptchaStorePayload{Scene: "login", Dots: []clickCaptchaTarget{{Index: 1, X: 100, Y: 100, Width: 10, Height: 10}, {Index: 0, X: 0, Y: 0, Width: 10, Height: 10}}})
			if err := client.Set(t.Context(), constants.ClickCaptchaRedisKeyPrefix+"token", payload, time.Minute).Err(); err != nil {
				t.Fatal(err)
			}
			ok, err := captcha.Verify(t.Context(), "token", test.points)
			if err != nil || ok != test.want {
				t.Fatalf("ok=%t err=%v want=%t", ok, err, test.want)
			}
			if ok, err := captcha.Verify(t.Context(), "token", test.points); err == nil || ok {
				t.Fatal("captcha reused")
			}
		})
	}
}
func TestEmailGeneratorUsesFullAlphabetAndUniqueIDs(t *testing.T) {
	generator := NewEmailGenerator()
	seen := map[string]bool{}
	for range 100 {
		id, code := generator.Generate(6, "0123456789")
		if len(id) != 64 || len(code) != 6 || seen[id] {
			t.Fatalf("invalid generated values: id=%q code=%q", id, code)
		}
		seen[id] = true
		for _, ch := range code {
			if !strings.ContainsRune("0123456789", ch) {
				t.Fatal("character outside alphabet")
			}
		}
	}
	if id, code := generator.Generate(6, ""); id != "" || code != "" {
		t.Fatal("empty alphabet must fail closed")
	}
}
