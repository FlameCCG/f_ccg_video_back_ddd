package video

import (
	"sync"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
)

const (
	// authorStatsCacheTTL 作者作品聚合统计的进程内缓存 TTL（与原 userVideoStatsCacheTTL 一致）。
	authorStatsCacheTTL = 10 * time.Second
	// usernameCacheTTL 用户 ID → 用户名的进程内缓存 TTL（与原 userUsernameCacheTTL 一致）。
	usernameCacheTTL = 5 * time.Minute
)

// AuthorStatsCache 作者作品聚合统计的进程内短 TTL 缓存
// （原 service/video_service/user_stat_cache.go 的包级 sync.Map，改为可注入实例）。
type AuthorStatsCache struct {
	items sync.Map
	ttl   time.Duration
}

var _ app.AuthorStatsCache = (*AuthorStatsCache)(nil)

// NewAuthorStatsCache 构造作者统计缓存。
func NewAuthorStatsCache() *AuthorStatsCache {
	return &AuthorStatsCache{ttl: authorStatsCacheTTL}
}

// authorStatsEntry 单条作者统计缓存项。
type authorStatsEntry struct {
	stats     app.UserVideoStats
	expiresAt time.Time
}

// Load 读取未过期的作者统计；未命中返回 (零值, false)。
func (c *AuthorStatsCache) Load(authorUsername string) (app.UserVideoStats, bool) {
	value, ok := c.items.Load(authorUsername)
	if !ok {
		return app.UserVideoStats{}, false
	}

	entry, ok := value.(authorStatsEntry)
	if !ok || time.Now().After(entry.expiresAt) {
		c.items.Delete(authorUsername)

		return app.UserVideoStats{}, false
	}

	return entry.stats, true
}

// Store 写入作者统计缓存。
func (c *AuthorStatsCache) Store(authorUsername string, stats app.UserVideoStats) {
	if authorUsername == "" {
		return
	}

	c.items.Store(authorUsername, authorStatsEntry{
		stats:     stats,
		expiresAt: time.Now().Add(c.ttl),
	})
}

// UsernameCache 用户 ID → 用户名的进程内缓存
// （原 service/video_service/user_video_service.go 的包级 userUsernameCache，改为可注入实例）。
type UsernameCache struct {
	items sync.Map
	ttl   time.Duration
}

var _ app.UsernameCache = (*UsernameCache)(nil)

// NewUsernameCache 构造用户名缓存。
func NewUsernameCache() *UsernameCache {
	return &UsernameCache{ttl: usernameCacheTTL}
}

// usernameEntry 单条用户名缓存项。
type usernameEntry struct {
	username  string
	expiresAt time.Time
}

// Load 读取未过期的用户名；未命中或空串返回 ("", false)。
func (c *UsernameCache) Load(userID uint) (string, bool) {
	if userID == 0 {
		return "", false
	}

	value, ok := c.items.Load(userID)
	if !ok {
		return "", false
	}

	entry, ok := value.(usernameEntry)
	if !ok {
		c.items.Delete(userID)

		return "", false
	}

	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.items.Delete(userID)

		return "", false
	}

	return entry.username, entry.username != ""
}

// Store 写入用户名缓存。
func (c *UsernameCache) Store(userID uint, username string) {
	if userID == 0 || username == "" {
		return
	}

	c.items.Store(userID, usernameEntry{
		username:  username,
		expiresAt: time.Now().Add(c.ttl),
	})
}

// Delete 删除用户名缓存。
func (c *UsernameCache) Delete(userID uint) {
	if userID == 0 {
		return
	}

	c.items.Delete(userID)
}
