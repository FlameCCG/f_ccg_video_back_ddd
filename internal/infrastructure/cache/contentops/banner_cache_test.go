package contentops

import (
	"testing"
	"time"

	contentopsdomain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"

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
	cache := NewBannerCache(client, zap.NewNop())

	homePartition := uint(0)
	items := []contentopsdomain.Banner{
		*contentopsdomain.RestoreBanner(contentopsdomain.BannerSnapshot{ID: 2, Cover: "b", Show: true, Type: enum.BannerTypeHome}),
		*contentopsdomain.RestoreBanner(contentopsdomain.BannerSnapshot{ID: 1, Cover: "a", Show: true, Type: enum.BannerTypeHome}),
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

	if len(loaded) != 2 || loaded[0].ID() != 2 || loaded[1].Cover() != "a" {
		t.Fatalf("unexpected cached items: %+v", loaded)
	}

	raw, err := server.Get(key)
	if err != nil {
		t.Fatalf("read cache payload: %v", err)
	}
	const wantPayload = `[{"id":2,"cover":"b","href":"","show":true,"type":1,"partitionId":0},{"id":1,"cover":"a","href":"","show":true,"type":1,"partitionId":0}]`
	if raw != wantPayload {
		t.Fatalf("cache payload changed: %s", raw)
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
	cache := NewBannerCache(client, zap.NewNop())

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

func TestBannerCacheLoadsLegacyNullAsEmptyList(t *testing.T) {
	server, client := newTestRedis(t)
	cache := NewBannerCache(client, zap.NewNop())
	if err := server.Set("banner:visible:1:0", "null"); err != nil {
		t.Fatal(err)
	}
	partition := uint(0)
	loaded, ok := cache.LoadVisible(enum.BannerTypeHome, &partition)
	if !ok || loaded == nil || len(loaded) != 0 {
		t.Fatalf("legacy null: hit=%v list=%#v, want non-nil empty list", ok, loaded)
	}
}

func TestBannerCacheLoadsLegacyPayload(t *testing.T) {
	server, client := newTestRedis(t)
	cache := NewBannerCache(client, zap.NewNop())

	key := "banner:visible:1:0"
	payload := `[{"id":2,"cover":"b","href":"","show":true,"type":1,"partitionId":7},{"id":1,"cover":"","href":"","show":false,"type":3,"partitionId":0}]`
	if err := server.Set(key, payload); err != nil {
		t.Fatalf("seed legacy cache failed: %v", err)
	}

	homePartition := uint(0)
	loaded, ok := cache.LoadVisible(enum.BannerTypeHome, &homePartition)
	if !ok {
		t.Fatal("legacy cache should hit")
	}

	if len(loaded) != 2 || loaded[0].ID() != 2 || loaded[0].Cover() != "b" || !loaded[0].Show() ||
		loaded[0].Type() != enum.BannerTypeHome || loaded[0].PartitionID() != 7 || loaded[0].Href() != "" {
		t.Fatalf("unexpected first legacy banner: %+v", loaded[0].Snapshot())
	}

	if loaded[1].ID() != 1 || loaded[1].Cover() != "" || loaded[1].Show() || loaded[1].Type() != enum.BannerTypeUser || loaded[1].PartitionID() != 0 {
		t.Fatalf("unexpected empty-value legacy banner: %+v", loaded[1].Snapshot())
	}
}

// TestBannerCacheWithoutRedis 保证 Redis 缺失时缓存操作静默降级。
func TestBannerCacheWithoutRedis(t *testing.T) {
	cache := NewBannerCache(nil, zap.NewNop())

	if _, ok := cache.LoadVisible(enum.BannerTypeHome, nil); ok {
		t.Fatal("nil redis should always miss")
	}

	cache.StoreVisible(enum.BannerTypeHome, nil, nil)
	cache.InvalidateVisible()
}
