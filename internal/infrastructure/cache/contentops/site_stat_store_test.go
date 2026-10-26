package contentops

import (
	"context"
	"crypto/sha1" //nolint:gosec // Legacy non-authentication digest format must remain compatible with stored identifiers.
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSiteStatStoreTouchAndHeartbeat(t *testing.T) {
	t.Parallel()

	server, client := newTestRedis(t)
	store := NewSiteStatStore(client)
	ctx := context.Background()

	day := time.Now().Format("20060102")

	result, isFirstToday, err := store.Touch(ctx, 5, "203.0.113.10")
	require.NoError(t, err, "touch failed: %v", err)

	require.EqualValues(t, 1, result.PV, "unexpected touch result: %+v", result)
	require.EqualValues(t, 1, result.UV, "unexpected touch result: %+v", result)
	require.EqualValues(t, 1, result.Online, "unexpected touch result: %+v", result)

	require.True(t, isFirstToday, "first touch of a logged-in user should report first-today")

	// key 格式属于外部契约
	for _, key := range []string{
		"site:stat:pv:" + day,
		"site:stat:uv:visit:" + day,
		"site:stat:uv:login:" + day,
		"site:stat:online:users",
	} {
		require.True(t, server.Exists(key), "expected key %s", key)
	}

	// PV 永久保留（不设置过期），UV 集合保留 8 天
	{
		ttl := server.TTL("site:stat:pv:" + day)
		require.EqualValues(t, 0, ttl, "pv key should not expire, got ttl %v", ttl)
	}

	{
		ttl := server.TTL("site:stat:uv:visit:" + day)
		require.Equal(t, 8*24*time.Hour, ttl, "unexpected visit uv ttl: %v", ttl)
	}

	// 登录用户成员为 u:<uid>
	members, err := server.Members("site:stat:uv:login:" + day)
	require.NoError(t, err, "read login uv members failed: %v", err)

	require.Len(t, members, 1, "unexpected login uv members: %v", members)
	require.Equal(t, "u:5", members[0], "unexpected login uv members: %v", members)

	// 同一用户再次打点：PV 自增、当日首次标记为 false
	result, isFirstToday, err = store.Touch(ctx, 5, "203.0.113.10")
	require.NoError(t, err, "second touch failed: %v", err)

	require.EqualValues(t, 2, result.PV, "unexpected second touch result: %+v", result)
	require.EqualValues(t, 1, result.UV, "unexpected second touch result: %+v", result)

	require.False(t, isFirstToday, "second touch should not report first-today")

	// 游客心跳：成员为 v:<sha1(ip)>，不增加 PV
	heartbeat, err := store.Heartbeat(ctx, 0, "198.51.100.20")
	require.NoError(t, err, "heartbeat failed: %v", err)

	require.EqualValues(t, 2, heartbeat.UV, "unexpected heartbeat result: %+v", heartbeat)
	require.EqualValues(t, 2, heartbeat.Online, "unexpected heartbeat result: %+v", heartbeat)

	pv, err := server.Get("site:stat:pv:" + day)
	require.NoError(t, err, "read pv failed: %v", err)

	require.Equal(t, "2", pv, "heartbeat must not bump pv, got %s", pv)

	//nolint:gosec // Compatibility checksum only; authentication and passwords use stronger primitives.
	sum := sha1.Sum([]byte("198.51.100.20"))
	wantMember := "v:" + hex.EncodeToString(sum[:])

	{
		score, err := server.ZScore("site:stat:online:users", wantMember)
		require.NoError(t, err, "visitor member %s should be online (score=%v err=%v)", wantMember, score, err)
		require.NotEqualValues(t, 0, score, "visitor member %s should be online (score=%v err=%v)", wantMember, score, err)
	}

	online, err := store.OnlineCount(ctx)
	require.NoError(t, err, "online count failed: %v", err)

	require.EqualValues(t, 2, online, "unexpected online count: %d", online)

	uv, err := store.DailyUV(ctx, time.Now())
	require.NoError(t, err, "daily uv failed: %v", err)

	require.EqualValues(t, 2, uv, "unexpected daily uv: %d", uv)
}

// TestSiteStatStoreUnidentifiedVisitor 既无 uid 也无 IP 时报错（文案属外部契约）。
func TestSiteStatStoreUnidentifiedVisitor(t *testing.T) {
	t.Parallel()

	_, client := newTestRedis(t)
	store := NewSiteStatStore(client)
	ctx := context.Background()

	{
		_, _, err := store.Touch(ctx, 0, "")
		require.Error(t, err, "unexpected touch error: %v", err)
		require.Equal(t, "无法识别访客", err.Error(), "unexpected touch error: %v", err)
	}

	{
		_, err := store.Heartbeat(ctx, 0, "")
		require.Error(t, err, "unexpected heartbeat error: %v", err)
		require.Equal(t, "无法识别访客", err.Error(), "unexpected heartbeat error: %v", err)
	}
}

// TestSiteStatStoreWithoutRedis Redis 缺失时返回与原实现一致的错误文案。
func TestSiteStatStoreWithoutRedis(t *testing.T) {
	t.Parallel()

	store := NewSiteStatStore(nil)
	ctx := context.Background()

	{
		_, _, err := store.Touch(ctx, 1, "1.1.1.1")
		require.Error(t, err, "unexpected touch error: %v", err)
		require.Equal(t, testRedisUninitialized, err.Error(), "unexpected touch error: %v", err)
	}

	{
		_, err := store.Heartbeat(ctx, 1, "1.1.1.1")
		require.Error(t, err, "unexpected heartbeat error: %v", err)
		require.Equal(t, testRedisUninitialized, err.Error(), "unexpected heartbeat error: %v", err)
	}

	{
		_, err := store.OnlineCount(ctx)
		require.Error(t, err, "unexpected online count error: %v", err)
		require.Equal(t, testRedisUninitialized, err.Error(), "unexpected online count error: %v", err)
	}

	{
		_, err := store.DailyUV(ctx, time.Now())
		require.Error(t, err, "unexpected daily uv error: %v", err)
		require.Equal(t, testRedisUninitialized, err.Error(), "unexpected daily uv error: %v", err)
	}
}
