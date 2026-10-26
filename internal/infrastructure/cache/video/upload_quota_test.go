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
	"github.com/stretchr/testify/require"
)

func TestUploadQuotaAtomicAndIdempotent(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})

	t.Cleanup(func() { _ = client.Close() })

	store := NewUploadQuotaStore(client)
	ctx := context.Background()

	var (
		allowed atomic.Int32
		wg      sync.WaitGroup
	)
	for i := range 30 {
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

	{
		got := allowed.Load()
		require.EqualValues(t, 10, got, "allowed %d distinct hashes, want 10", got)
	}

	for range 10 {
		ok, err := store.Claim(ctx, 7, "hash-0", 10)
		require.NoError(t, err)
		// hash-0 might be outside the first ten winners, so do not assert here.
		_ = ok
	}

	other, err := store.Claim(ctx, 8, "another", 10)
	require.NoError(t, err, "different user must have own quota: %v, %v", other, err)
	require.True(t, other, "different user must have own quota: %v, %v", other, err)

	keys := server.Keys()
	require.Len(t, keys, 2, "quota keys = %v", keys)

	for _, key := range keys {
		if key[:len("video:upload:daily:7:")] == "video:upload:daily:7:" {
			members, err := client.SMembers(ctx, key).Result()
			require.NoError(t, err, "members = %v, %v", members, err)
			require.Len(t, members, 10, "members = %v, %v", members, err)

			ok, err := store.Claim(ctx, 7, members[0], 10)
			require.NoError(t, err, "same hash must remain allowed: %v, %v", ok, err)
			require.True(t, ok, "same hash must remain allowed: %v, %v", ok, err)
		}
	}

	for _, key := range keys {
		{
			ttl := server.TTL(key)
			require.Positive(t, ttl, "key %s TTL = %v", key, ttl)
			require.LessOrEqual(t, ttl, 24*time.Hour, "key %s TTL = %v", key, ttl)
		}
	}
}

func TestUploadQuotaExpiresAtBeijingMidnight(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})

	t.Cleanup(func() { _ = client.Close() })

	store := NewUploadQuotaStore(client)
	beijing := time.FixedZone("Beijing", 8*3600)
	server.SetTime(time.Date(2026, 9, 24, 23, 59, 59, 0, beijing))

	ok, err := store.Claim(t.Context(), 1, "same", 1)
	require.NoError(t, err, "before midnight: %v, %v", ok, err)
	require.True(t, ok, "before midnight: %v, %v", ok, err)

	keys := server.Keys()
	require.Len(t, keys, 1, "keys before midnight: %v", keys)

	{
		ttl := server.TTL(keys[0])
		require.Positive(t, ttl, "TTL = %v, want one second", ttl)
		require.LessOrEqual(t, ttl, time.Second, "TTL = %v, want one second", ttl)
	}

	server.FastForward(2 * time.Second)

	ok, err = store.Claim(t.Context(), 1, "same", 1)
	require.NoError(t, err, "same hash on next day: %v, %v", ok, err)
	require.True(t, ok, "same hash on next day: %v, %v", ok, err)

	{
		keys := server.Keys()
		require.Len(t, keys, 1, "keys after midnight: %v", keys)
	}

	ok, err = store.Claim(t.Context(), 1, "new", 1)
	require.NoError(t, err, "next day's first request must consume its slot: %v, %v", ok, err)
	require.False(t, ok, "next day's first request must consume its slot: %v, %v", ok, err)
}

func TestUploadQuotaSameHashAtLimit(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})

	t.Cleanup(func() { _ = client.Close() })

	store := NewUploadQuotaStore(client)
	for _, hash := range []string{"one", "one", "two"} {
		ok, err := store.Claim(t.Context(), 1, hash, 1)
		require.NoError(t, err)

		require.Equal(t, ok, (hash == "one"), "hash %q allowed = %v", hash, ok)
	}
}

func TestUploadQuotaTenthAndEleventhFile(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})

	t.Cleanup(func() { _ = client.Close() })

	store := NewUploadQuotaStore(client)
	for i := 1; i <= 11; i++ {
		allowed, err := store.Claim(t.Context(), 42, fmt.Sprintf("source-%d", i), 10)
		require.NoError(t, err)

		require.Equal(t, allowed, (i <= 10), "file %d allowed = %v", i, allowed)
	}
}

func TestUploadQuotaRedisUnavailable(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { require.NoError(t, client.Close()) }()

	server.Close()

	{
		_, err := NewUploadQuotaStore(client).Claim(t.Context(), 1, "hash", 10)
		require.Error(t, err, "Redis failure must reject the claim")
	}
}
