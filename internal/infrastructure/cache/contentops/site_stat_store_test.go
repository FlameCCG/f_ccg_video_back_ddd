package contentops

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"testing"
	"time"
)

func TestSiteStatStoreTouchAndHeartbeat(t *testing.T) {
	server, client := newTestRedis(t)
	store := NewSiteStatStore(client)
	ctx := context.Background()

	day := time.Now().Format("20060102")

	result, isFirstToday, err := store.Touch(ctx, 5, "203.0.113.10")
	if err != nil {
		t.Fatalf("touch failed: %v", err)
	}

	if result.PV != 1 || result.UV != 1 || result.Online != 1 {
		t.Fatalf("unexpected touch result: %+v", result)
	}

	if !isFirstToday {
		t.Fatal("first touch of a logged-in user should report first-today")
	}

	// key 格式属于外部契约
	for _, key := range []string{
		"site:stat:pv:" + day,
		"site:stat:uv:visit:" + day,
		"site:stat:uv:login:" + day,
		"site:stat:online:users",
	} {
		if !server.Exists(key) {
			t.Fatalf("expected key %s", key)
		}
	}

	// PV 永久保留（不设置过期），UV 集合保留 8 天
	if ttl := server.TTL("site:stat:pv:" + day); ttl != 0 {
		t.Fatalf("pv key should not expire, got ttl %v", ttl)
	}

	if ttl := server.TTL("site:stat:uv:visit:" + day); ttl != 8*24*time.Hour {
		t.Fatalf("unexpected visit uv ttl: %v", ttl)
	}

	// 登录用户成员为 u:<uid>
	members, err := server.Members("site:stat:uv:login:" + day)
	if err != nil {
		t.Fatalf("read login uv members failed: %v", err)
	}

	if len(members) != 1 || members[0] != "u:5" {
		t.Fatalf("unexpected login uv members: %v", members)
	}

	// 同一用户再次打点：PV 自增、当日首次标记为 false
	result, isFirstToday, err = store.Touch(ctx, 5, "203.0.113.10")
	if err != nil {
		t.Fatalf("second touch failed: %v", err)
	}

	if result.PV != 2 || result.UV != 1 {
		t.Fatalf("unexpected second touch result: %+v", result)
	}

	if isFirstToday {
		t.Fatal("second touch should not report first-today")
	}

	// 游客心跳：成员为 v:<sha1(ip)>，不增加 PV
	heartbeat, err := store.Heartbeat(ctx, 0, "198.51.100.20")
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}

	if heartbeat.UV != 2 || heartbeat.Online != 2 {
		t.Fatalf("unexpected heartbeat result: %+v", heartbeat)
	}

	pv, err := server.Get("site:stat:pv:" + day)
	if err != nil {
		t.Fatalf("read pv failed: %v", err)
	}

	if pv != "2" {
		t.Fatalf("heartbeat must not bump pv, got %s", pv)
	}

	sum := sha1.Sum([]byte("198.51.100.20"))
	wantMember := "v:" + hex.EncodeToString(sum[:])

	if score, err := server.ZScore("site:stat:online:users", wantMember); err != nil || score == 0 {
		t.Fatalf("visitor member %s should be online (score=%v err=%v)", wantMember, score, err)
	}

	online, err := store.OnlineCount(ctx)
	if err != nil {
		t.Fatalf("online count failed: %v", err)
	}

	if online != 2 {
		t.Fatalf("unexpected online count: %d", online)
	}

	uv, err := store.DailyUV(ctx, time.Now())
	if err != nil {
		t.Fatalf("daily uv failed: %v", err)
	}

	if uv != 2 {
		t.Fatalf("unexpected daily uv: %d", uv)
	}
}

// TestSiteStatStoreUnidentifiedVisitor 既无 uid 也无 IP 时报错（文案属外部契约）。
func TestSiteStatStoreUnidentifiedVisitor(t *testing.T) {
	_, client := newTestRedis(t)
	store := NewSiteStatStore(client)
	ctx := context.Background()

	if _, _, err := store.Touch(ctx, 0, ""); err == nil || err.Error() != "无法识别访客" {
		t.Fatalf("unexpected touch error: %v", err)
	}

	if _, err := store.Heartbeat(ctx, 0, ""); err == nil || err.Error() != "无法识别访客" {
		t.Fatalf("unexpected heartbeat error: %v", err)
	}
}

// TestSiteStatStoreWithoutRedis Redis 缺失时返回与原实现一致的错误文案。
func TestSiteStatStoreWithoutRedis(t *testing.T) {
	store := NewSiteStatStore(nil)
	ctx := context.Background()

	if _, _, err := store.Touch(ctx, 1, "1.1.1.1"); err == nil || err.Error() != "redis 未初始化" {
		t.Fatalf("unexpected touch error: %v", err)
	}

	if _, err := store.Heartbeat(ctx, 1, "1.1.1.1"); err == nil || err.Error() != "redis 未初始化" {
		t.Fatalf("unexpected heartbeat error: %v", err)
	}

	if _, err := store.OnlineCount(ctx); err == nil || err.Error() != "redis 未初始化" {
		t.Fatalf("unexpected online count error: %v", err)
	}

	if _, err := store.DailyUV(ctx, time.Now()); err == nil || err.Error() != "redis 未初始化" {
		t.Fatalf("unexpected daily uv error: %v", err)
	}
}
