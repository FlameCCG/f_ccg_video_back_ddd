package search

import (
	"context"
	"f_ccg_video_back_ddd/pkg/constants"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// 对应原 service/search_service/hot_keywords.go 的 Redis 访问部分。
// Redis key（search:hot:keywords:zset）与「只增不过期」的语义属于外部契约。

// newTestRedis 启动 miniredis 并返回连接其上的客户端（离线测试，不依赖真实 Redis）。
func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})

	t.Cleanup(func() { _ = client.Close() })

	return server, client
}

// TestHotKeywordStoreUsesContractKey ZINCRBY 落在约定的榜单 key 上，且不设过期。
func TestHotKeywordStoreUsesContractKey(t *testing.T) {
	t.Parallel()

	server, client := newTestRedis(t)
	store := NewHotKeywordStore(client)
	ctx := context.Background()

	store.Incr(ctx, "教程")
	store.Incr(ctx, "教程")
	store.Incr(ctx, "vlog")

	score, err := server.ZScore(constants.SearchHotKeywordZSetKey, "教程")
	if err != nil {
		t.Fatalf("ZScore: %v", err)
	}

	if score != 2 {
		t.Fatalf("score = %v, want 2", score)
	}

	// 榜单不设 TTL（由运维侧决定何时重置）。
	if ttl := server.TTL(constants.SearchHotKeywordZSetKey); ttl != 0 {
		t.Fatalf("hot keyword zset must not expire, ttl = %v", ttl)
	}
}

func TestHotKeywordStoreCardAndRevRange(t *testing.T) {
	t.Parallel()

	_, client := newTestRedis(t)
	store := NewHotKeywordStore(client)
	ctx := context.Background()

	for _, kw := range []string{"a", "a", "a", "b", "b", "c"} {
		store.Incr(ctx, kw)
	}

	total, err := store.Card(ctx)
	if err != nil {
		t.Fatalf("Card: %v", err)
	}

	if total != 3 {
		t.Fatalf("card = %d, want 3", total)
	}

	entries, err := store.RevRangeWithScores(ctx, 0, 1)
	if err != nil {
		t.Fatalf("RevRangeWithScores: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("entries = %+v, want 2", entries)
	}

	if entries[0].Member != "a" || entries[0].Score != 3 {
		t.Fatalf("first entry = %+v", entries[0])
	}

	if entries[1].Member != "b" || entries[1].Score != 2 {
		t.Fatalf("second entry = %+v", entries[1])
	}
}

// TestHotKeywordStoreMissingKeyReturnsZero key 不存在时 ZCARD 返回 0、
// ZREVRANGE 返回空集合，均不报错（应用层据此回退）。
func TestHotKeywordStoreMissingKeyReturnsZero(t *testing.T) {
	t.Parallel()

	_, client := newTestRedis(t)
	store := NewHotKeywordStore(client)
	ctx := context.Background()

	total, err := store.Card(ctx)
	if err != nil {
		t.Fatalf("Card: %v", err)
	}

	if total != 0 {
		t.Fatalf("card = %d, want 0", total)
	}

	entries, err := store.RevRangeWithScores(ctx, 0, 9)
	if err != nil {
		t.Fatalf("RevRangeWithScores: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("entries = %+v, want none", entries)
	}
}

// TestHotKeywordStoreDegradesWithoutRedis client 为 nil 时全部方法安全降级，
// 与原 global.RedisClient == nil 的行为一致。
func TestHotKeywordStoreDegradesWithoutRedis(t *testing.T) {
	t.Parallel()

	store := NewHotKeywordStore(nil)
	if store.Available() {
		t.Fatal("nil client must report unavailable")
	}

	ctx := context.Background()

	if total, err := store.Card(ctx); err != nil || total != 0 {
		t.Fatalf("Card = %d, %v", total, err)
	}

	if entries, err := store.RevRangeWithScores(ctx, 0, 9); err != nil || entries != nil {
		t.Fatalf("RevRangeWithScores = %+v, %v", entries, err)
	}

	// 不应 panic。
	store.Incr(ctx, "教程")
}

// TestHotKeywordStoreSwallowsIncrErrors 热度累加是 best-effort：
// Redis 不可用时静默忽略，不影响搜索主流程。
func TestHotKeywordStoreSwallowsIncrErrors(t *testing.T) {
	t.Parallel()

	server, client := newTestRedis(t)
	store := NewHotKeywordStore(client)

	server.Close()

	store.Incr(context.Background(), "教程")
}
