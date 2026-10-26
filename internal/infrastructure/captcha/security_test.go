package captcha

import (
	"encoding/json"
	"f_ccg_video_back_ddd/pkg/constants"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/identity"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func TestClickCaptchaRejectsRepeatedOrReorderedTargets(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
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

			payload, _ := json.Marshal(clickCaptchaStorePayload{Scene: captchaLogin, Dots: []clickCaptchaTarget{{Index: 1, X: 100, Y: 100, Width: 10, Height: 10}, {Index: 0, X: 0, Y: 0, Width: 10, Height: 10}}})
			{
				err := client.Set(t.Context(), constants.ClickCaptchaRedisKeyPrefix+"token", payload, time.Minute).Err()
				require.NoError(t, err)
			}

			ok, err := captcha.Verify(t.Context(), "token", test.points)
			require.NoError(t, err, "ok=%t err=%v want=%t", ok, err, test.want)
			require.Equal(t, test.want, ok, "ok=%t err=%v want=%t", ok, err, test.want)

			{
				ok, err := captcha.Verify(t.Context(), "token", test.points)
				require.Error(t, err, "captcha reused")
				require.False(t, ok, "captcha reused")
			}
		})
	}
}

func TestEmailGeneratorUsesFullAlphabetAndUniqueIDs(t *testing.T) {
	t.Parallel()

	generator := NewEmailGenerator()
	seen := map[string]bool{}

	for range 100 {
		id, code := generator.Generate(6, "0123456789")
		require.Len(t, id, 64, "invalid generated values: id=%q code=%q", id, code)
		require.Len(t, code, 6, "invalid generated values: id=%q code=%q", id, code)
		require.False(t, seen[id], "invalid generated values: id=%q code=%q", id, code)

		seen[id] = true

		for _, ch := range code {
			require.True(t, strings.ContainsRune("0123456789", ch), "character outside alphabet")
		}
	}

	{
		id, code := generator.Generate(6, "")
		require.Empty(t, id, "empty alphabet must fail closed")
		require.Empty(t, code, "empty alphabet must fail closed")
	}
}
