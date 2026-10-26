package contentops

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/contentops"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// newTestRedis 启动 miniredis 并返回连接其上的客户端（离线测试，不依赖真实 Redis）。
func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})

	t.Cleanup(func() { _ = client.Close() })

	return server, client
}

func TestVisibleBannerCacheKey(t *testing.T) {
	t.Parallel()

	partitionID := uint(7)

	cases := map[string]string{
		visibleBannerCacheKey(enum.BannerTypeHome, nil):          "banner:visible:1:nil",
		visibleBannerCacheKey(enum.BannerTypeHome, &partitionID): "banner:visible:1:7",
		visibleBannerCacheKey(enum.BannerTypeUser, nil):          "banner:visible:3:nil",
	}
	for got, want := range cases {
		require.Equal(t, want, got, "unexpected cache key: got %s want %s", got, want)
	}
}

func TestBannerCacheStoreLoadInvalidate(t *testing.T) {
	t.Parallel()

	server, client := newTestRedis(t)
	cache := NewBannerCache(client, zap.NewNop())

	homePartition := uint(0)
	items := []domain.Banner{
		{ID: 2, Cover: "b", Show: true, Type: enum.BannerTypeHome},
		{ID: 1, Cover: "a", Show: true, Type: enum.BannerTypeHome},
	}

	cache.StoreVisible(enum.BannerTypeHome, &homePartition, items)

	key := "banner:visible:1:0"
	require.True(t, server.Exists(key), "cache key %s should exist", key)

	{
		ttl := server.TTL(key)
		require.Equal(t, 10*time.Minute, ttl, "unexpected cache ttl: %v", ttl)
	}

	loaded, ok := cache.LoadVisible(enum.BannerTypeHome, &homePartition)
	require.True(t, ok, "cache should hit")

	require.Len(t, loaded, 2, "unexpected cached items: %+v", loaded)
	require.EqualValues(t, 2, loaded[0].ID, "unexpected cached items: %+v", loaded)
	require.Equal(t, "a", loaded[1].Cover, "unexpected cached items: %+v", loaded)

	// 缓存另一个 key，验证按前缀整体失效
	cache.StoreVisible(enum.BannerTypeTop, nil, items[:1])

	require.True(t, server.Exists("banner:visible:2:nil"), "second cache key should exist")

	cache.InvalidateVisible()

	require.False(t, server.Exists(key), "invalidate should drop every banner:visible: key")
	require.False(t, server.Exists("banner:visible:2:nil"), "invalidate should drop every banner:visible: key")

	{
		_, ok := cache.LoadVisible(enum.BannerTypeHome, &homePartition)
		require.False(t, ok, "cache should miss after invalidate")
	}
}

func TestBannerCacheDropsCorruptedValue(t *testing.T) {
	t.Parallel()

	server, client := newTestRedis(t)
	cache := NewBannerCache(client, zap.NewNop())

	key := "banner:visible:1:nil"
	{
		err := server.Set(key, "not-json")
		require.NoError(t, err, "seed corrupted cache failed: %v", err)
	}

	{
		_, ok := cache.LoadVisible(enum.BannerTypeHome, nil)
		require.False(t, ok, "corrupted cache should miss")
	}

	require.False(t, server.Exists(key), "corrupted cache key should be deleted")
}

// TestBannerCacheWithoutRedis 保证 Redis 缺失时缓存操作静默降级（与原实现一致）。
func TestBannerCacheWithoutRedis(t *testing.T) {
	t.Parallel()

	cache := NewBannerCache(nil, zap.NewNop())

	{
		_, ok := cache.LoadVisible(enum.BannerTypeHome, nil)
		require.False(t, ok, "nil redis should always miss")
	}

	cache.StoreVisible(enum.BannerTypeHome, nil, nil)
	cache.InvalidateVisible()
}
