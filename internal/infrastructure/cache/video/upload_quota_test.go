package video

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestUploadQuotaAtomicAndIdempotent(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewUploadQuotaStore(client)
	ctx := context.Background()
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, err := store.Claim(ctx, 7, fmt.Sprintf("hash-%d", i), 10)
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			if ok {
				allowed.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := allowed.Load(); got != 10 {
		t.Fatalf("allowed %d distinct hashes, want 10", got)
	}
	for i := 0; i < 10; i++ {
		ok, err := store.Claim(ctx, 7, "hash-0", 10)
		if err != nil {
			t.Fatal(err)
		}
		// hash-0 might be outside the first ten winners, so do not assert here.
		_ = ok
	}
	other, err := store.Claim(ctx, 8, "another", 10)
	if err != nil || !other {
		t.Fatalf("different user must have own quota: %v, %v", other, err)
	}
	keys := server.Keys()
	if len(keys) != 2 {
		t.Fatalf("quota keys = %v", keys)
	}
	for _, key := range keys {
		if key[:len("video:upload:daily:7:")] == "video:upload:daily:7:" {
			members, err := client.SMembers(ctx, key).Result()
			if err != nil || len(members) != 10 {
				t.Fatalf("members = %v, %v", members, err)
			}
			ok, err := store.Claim(ctx, 7, members[0], 10)
			if err != nil || !ok {
				t.Fatalf("same hash must remain allowed: %v, %v", ok, err)
			}
		}
	}
	for _, key := range keys {
		if ttl := server.TTL(key); ttl <= 0 || ttl > 24*time.Hour {
			t.Fatalf("key %s TTL = %v", key, ttl)
		}
	}
}

func TestUploadQuotaExpiresAtBeijingMidnight(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewUploadQuotaStore(client)
	beijing := time.FixedZone("Beijing", 8*3600)
	server.SetTime(time.Date(2026, 9, 24, 23, 59, 59, 0, beijing))
	ok, err := store.Claim(t.Context(), 1, "same", 1)
	if err != nil || !ok {
		t.Fatalf("before midnight: %v, %v", ok, err)
	}
	keys := server.Keys()
	if len(keys) != 1 {
		t.Fatalf("keys before midnight: %v", keys)
	}
	if ttl := server.TTL(keys[0]); ttl <= 0 || ttl > time.Second {
		t.Fatalf("TTL = %v, want one second", ttl)
	}
	server.FastForward(2 * time.Second)
	ok, err = store.Claim(t.Context(), 1, "same", 1)
	if err != nil || !ok {
		t.Fatalf("same hash on next day: %v, %v", ok, err)
	}
	if keys := server.Keys(); len(keys) != 1 {
		t.Fatalf("keys after midnight: %v", keys)
	}
	ok, err = store.Claim(t.Context(), 1, "new", 1)
	if err != nil || ok {
		t.Fatalf("next day's first request must consume its slot: %v, %v", ok, err)
	}
}

func TestUploadQuotaSameHashAtLimit(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewUploadQuotaStore(client)
	for _, hash := range []string{"one", "one", "two"} {
		ok, err := store.Claim(t.Context(), 1, hash, 1)
		if err != nil {
			t.Fatal(err)
		}
		if ok != (hash == "one") {
			t.Fatalf("hash %q allowed = %v", hash, ok)
		}
	}
}

func TestUploadQuotaTenthAndEleventhFile(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewUploadQuotaStore(client)
	for i := 1; i <= 11; i++ {
		allowed, err := store.Claim(t.Context(), 42, fmt.Sprintf("source-%d", i), 10)
		if err != nil {
			t.Fatal(err)
		}
		if allowed != (i <= 10) {
			t.Fatalf("file %d allowed = %v", i, allowed)
		}
	}
}

func TestUploadQuotaRedisUnavailable(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	server.Close()
	if _, err := NewUploadQuotaStore(client).Claim(t.Context(), 1, "hash", 10); err == nil {
		t.Fatal("Redis failure must reject the claim")
	}
}
