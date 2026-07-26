package contentops

import (
	"testing"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
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
	partitionID := uint(7)

	cases := map[string]string{
		visibleBannerCacheKey(enum.BannerTypeHome, nil):          "banner:visible:1:nil",
		visibleBannerCacheKey(enum.BannerTypeHome, &partitionID): "banner:visible:1:7",
		visibleBannerCacheKey(enum.BannerTypeUser, nil):          "banner:visible:3:nil",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("unexpected cache key: got %s want %s", got, want)
		}
	}
}

func TestBannerCacheStoreLoadInvalidate(t *testing.T) {
	server, client := newTestRedis(t)
	cache := NewBannerCache(client)

	homePartition := uint(0)
	items := []domain.Banner{
		{ID: 2, Cover: "b", Show: true, Type: enum.BannerTypeHome},
		{ID: 1, Cover: "a", Show: true, Type: enum.BannerTypeHome},
	}

	cache.StoreVisible(enum.BannerTypeHome, &homePartition, items)

	key := "banner:visible:1:0"
	if !server.Exists(key) {
		t.Fatalf("cache key %s should exist", key)
	}

	if ttl := server.TTL(key); ttl != 10*time.Minute {
		t.Fatalf("unexpected cache ttl: %v", ttl)
	}

	loaded, ok := cache.LoadVisible(enum.BannerTypeHome, &homePartition)
	if !ok {
		t.Fatal("cache should hit")
	}

	if len(loaded) != 2 || loaded[0].ID != 2 || loaded[1].Cover != "a" {
		t.Fatalf("unexpected cached items: %+v", loaded)
	}

	// 缓存另一个 key，验证按前缀整体失效
	cache.StoreVisible(enum.BannerTypeTop, nil, items[:1])

	if !server.Exists("banner:visible:2:nil") {
		t.Fatal("second cache key should exist")
	}

	cache.InvalidateVisible()

	if server.Exists(key) || server.Exists("banner:visible:2:nil") {
		t.Fatal("invalidate should drop every banner:visible: key")
	}

	if _, ok := cache.LoadVisible(enum.BannerTypeHome, &homePartition); ok {
		t.Fatal("cache should miss after invalidate")
	}
}

func TestBannerCacheDropsCorruptedValue(t *testing.T) {
	server, client := newTestRedis(t)
	cache := NewBannerCache(client)

	key := "banner:visible:1:nil"
	if err := server.Set(key, "not-json"); err != nil {
		t.Fatalf("seed corrupted cache failed: %v", err)
	}

	if _, ok := cache.LoadVisible(enum.BannerTypeHome, nil); ok {
		t.Fatal("corrupted cache should miss")
	}

	if server.Exists(key) {
		t.Fatal("corrupted cache key should be deleted")
	}
}

// TestBannerCacheWithoutRedis 保证 Redis 缺失时缓存操作静默降级（与原实现一致）。
func TestBannerCacheWithoutRedis(t *testing.T) {
	cache := NewBannerCache(nil)

	if _, ok := cache.LoadVisible(enum.BannerTypeHome, nil); ok {
		t.Fatal("nil redis should always miss")
	}

	cache.StoreVisible(enum.BannerTypeHome, nil, nil)
	cache.InvalidateVisible()
}
