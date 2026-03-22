package identity

import (
	"strconv"
	"testing"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func TestUserStatusMutationsPreserveRedisContract(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewUserStatusCache(client, zap.NewNop())
	statusKey := constants.UserStatusCacheKeyPrefix + "1"
	generationKey := constants.UserStatusGenKeyPrefix + "1"
	if err := server.Set(statusKey, strconv.Itoa(int(enum.UserStatusBanned))); err != nil {
		t.Fatal(err)
	}
	if err := server.Set(generationKey, "41"); err != nil {
		t.Fatal(err)
	}
	if status, ok := cache.Load(t.Context(), 1); !ok || status != enum.UserStatusBanned {
		t.Fatalf("existing cache is unreadable: status=%v hit=%v", status, ok)
	}

	cache.Write(1, enum.UserStatusActive)
	cache.StoreIfGenerationMatch(t.Context(), 1, enum.UserStatusBanned, 41)
	if status, ok := cache.Load(t.Context(), 1); !ok || status != enum.UserStatusActive {
		t.Fatalf("stale fill replaced authoritative write: status=%v hit=%v", status, ok)
	}
	if generation := cache.Generation(t.Context(), 1); generation != 42 {
		t.Fatalf("generation=%d, want 42", generation)
	}
	if server.TTL(statusKey) != 60*time.Second || server.TTL(generationKey) != 2*time.Minute {
		t.Fatalf("write changed TTLs: status=%v generation=%v", server.TTL(statusKey), server.TTL(generationKey))
	}

	cache.Invalidate(1)
	cache.StoreIfGenerationMatch(t.Context(), 1, enum.UserStatusBanned, 42)
	if _, ok := cache.Load(t.Context(), 1); ok {
		t.Fatal("old generation refilled invalidated status")
	}
	if generation := cache.Generation(t.Context(), 1); generation != 43 || server.TTL(generationKey) != 2*time.Minute {
		t.Fatalf("invalidated generation=%d TTL=%v, want 43 and 2m", generation, server.TTL(generationKey))
	}
	cache.StoreIfGenerationMatch(t.Context(), 1, enum.UserStatusActive, 43)
	if status, ok := cache.Load(t.Context(), 1); !ok || status != enum.UserStatusActive {
		t.Fatalf("current generation failed to refill: status=%v hit=%v", status, ok)
	}
	server.FastForward(time.Minute)
	if server.Exists(statusKey) || !server.Exists(generationKey) {
		t.Fatal("status and generation did not keep their existing expiration windows")
	}
	server.FastForward(time.Minute)
	if server.Exists(generationKey) {
		t.Fatal("generation did not expire after two minutes")
	}
}
