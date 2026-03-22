package notification

import (
	"encoding/json"
	"strings"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/notification"
)

func TestCountPayloadDecodesLegacyCache(t *testing.T) {
	legacy := []byte(`{"data":{"reply":1,"like":2,"at":3,"system":4,"message":5},"cachedAt":100}`)
	var payload redisValue
	if err := json.Unmarshal(legacy, &payload); err != nil {
		t.Fatal(err)
	}
	got := payload.Data.summary()
	want := app.NotificationCountSummary{Reply: 1, Like: 2, At: 3, System: 4, Message: 5}
	if got != want || payload.CachedAt != 100 {
		t.Fatalf("decoded %+v cachedAt=%d", got, payload.CachedAt)
	}

	raw, err := json.Marshal(redisValue{Data: (countPayload{}).fromSummary(app.NotificationCountSummary{}), CachedAt: 1})
	if err != nil {
		t.Fatal(err)
	}
	var again redisValue
	if err := json.Unmarshal(raw, &again); err != nil {
		t.Fatal(err)
	}
	if again.Data.summary() != (app.NotificationCountSummary{}) || again.CachedAt != 1 {
		t.Fatalf("zero payload changed: %s", raw)
	}
	encoded := string(raw)
	for _, key := range []string{`"reply":0`, `"like":0`, `"at":0`, `"system":0`, `"message":0`, `"cachedAt":1`} {
		if !strings.Contains(encoded, key) {
			t.Fatalf("missing %s in %s", key, encoded)
		}
	}
}
