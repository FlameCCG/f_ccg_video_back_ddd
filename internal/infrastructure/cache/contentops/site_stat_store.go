package contentops

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/contentops"

	"github.com/redis/go-redis/v9"
)

// 站点统计 Redis key 与窗口参数（属于外部契约，与原 service/site_service/stat_service.go 一致）。
const (
	siteDailyPVPrefix    = "site:stat:pv:"
	siteDailyVisitUVPref = "site:stat:uv:visit:"
	siteDailyLoginUVPref = "site:stat:uv:login:"
	siteOnlineUsersZSet  = "site:stat:online:users"
	siteDailyUVKeepDays  = 8
	onlineWindowSeconds  = 300 // 5min
)

// errRedisNotInitialized Redis 未初始化（文案与原实现一致，会直接回给客户端）。
var errRedisNotInitialized = errors.New("redis 未初始化")

// errVisitorUnidentified 既无登录用户也无可用 IP（文案与原实现一致）。
var errVisitorUnidentified = errors.New("无法识别访客")

// SiteStatStore 站点流量统计的 Redis 实现（原 service/site_service/stat_service.go）。
//
// PV 永久保留、UV 精确集合保留 8 天、在线用户按 5 分钟滑动窗口维护，
// 全部 key 格式、TTL 与 pipeline 命令序列与原实现完全一致。
type SiteStatStore struct {
	redis *redis.Client
}

var _ app.SiteStatStore = (*SiteStatStore)(nil)

// NewSiteStatStore 构造站点流量统计存储适配器。
func NewSiteStatStore(redisClient *redis.Client) *SiteStatStore {
	return &SiteStatStore{redis: redisClient}
}

// buildVisitorMember 游客成员标识：v: + IP 的 SHA1（避免明文 IP 落 Redis）。
func buildVisitorMember(ip string) string {
	if ip == "" {
		return ""
	}

	sum := sha1.Sum([]byte(ip))

	return "v:" + hex.EncodeToString(sum[:])
}

// buildUserMember 登录用户成员标识：u: + 用户 ID。
func buildUserMember(userID uint) string {
	if userID == 0 {
		return ""
	}

	return "u:" + strconv.FormatUint(uint64(userID), 10)
}

// resolveMember 优先按登录用户标识，其次按访客 IP 标识。
func resolveMember(userID uint, ip string) string {
	member := buildUserMember(userID)
	if member == "" {
		member = buildVisitorMember(ip)
	}

	return member
}

// Touch 站点打点：增加 PV、记录当日 UV（精确 set），维护在线用户（5min 窗口）。
//
// 第二个返回值表示登录用户是否为当日首次打点（供调用方发放每日登录奖励）。
func (s *SiteStatStore) Touch(ctx context.Context, userID uint, ip string) (*app.TouchResult, bool, error) {
	if s.redis == nil {
		return nil, false, errRedisNotInitialized
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	now := time.Now()
	dayKey := now.Format("20060102")
	pvKey := siteDailyPVPrefix + dayKey
	visitUVKey := siteDailyVisitUVPref + dayKey
	loginUVKey := siteDailyLoginUVPref + dayKey

	member := resolveMember(userID, ip)
	if member == "" {
		return nil, false, errVisitorUnidentified
	}

	pipe := s.redis.Pipeline()
	pvCmd := pipe.Incr(ctx, pvKey)
	pipe.Persist(ctx, pvKey) // PV 永久保留在 Redis（不设置过期）

	pipe.SAdd(ctx, visitUVKey, member)
	uvCountCmd := pipe.SCard(ctx, visitUVKey)
	pipe.Expire(ctx, visitUVKey, time.Hour*24*time.Duration(siteDailyUVKeepDays))

	var loginAddedCmd *redis.IntCmd
	if userID > 0 {
		loginAddedCmd = pipe.SAdd(ctx, loginUVKey, member)
		pipe.Expire(ctx, loginUVKey, time.Hour*24*time.Duration(siteDailyUVKeepDays))
	}

	cutoff := now.Add(-time.Second * time.Duration(onlineWindowSeconds)).Unix()
	pipe.ZRemRangeByScore(ctx, siteOnlineUsersZSet, "-inf", strconv.FormatInt(cutoff, 10))
	pipe.ZAdd(ctx, siteOnlineUsersZSet, redis.Z{Score: float64(now.Unix()), Member: member})
	onlineCmd := pipe.ZCount(ctx, siteOnlineUsersZSet, strconv.FormatInt(cutoff, 10), "+inf")

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, false, err
	}

	isFirstToday := userID > 0 && loginAddedCmd != nil && loginAddedCmd.Val() > 0

	return &app.TouchResult{
		PV:     pvCmd.Val(),
		UV:     uvCountCmd.Val(),
		Online: onlineCmd.Val(),
	}, isFirstToday, nil
}

// Heartbeat 在线心跳：不增加 PV，只记录当日 UV（精确 set），维护在线用户（5min 窗口）。
func (s *SiteStatStore) Heartbeat(ctx context.Context, userID uint, ip string) (*app.HeartbeatResult, error) {
	if s.redis == nil {
		return nil, errRedisNotInitialized
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	now := time.Now()
	dayKey := now.Format("20060102")
	visitUVKey := siteDailyVisitUVPref + dayKey

	member := resolveMember(userID, ip)
	if member == "" {
		return nil, errVisitorUnidentified
	}

	pipe := s.redis.Pipeline()
	pipe.SAdd(ctx, visitUVKey, member)
	uvCountCmd := pipe.SCard(ctx, visitUVKey)
	pipe.Expire(ctx, visitUVKey, time.Hour*24*time.Duration(siteDailyUVKeepDays))

	cutoff := now.Add(-time.Second * time.Duration(onlineWindowSeconds)).Unix()
	pipe.ZRemRangeByScore(ctx, siteOnlineUsersZSet, "-inf", strconv.FormatInt(cutoff, 10))
	pipe.ZAdd(ctx, siteOnlineUsersZSet, redis.Z{Score: float64(now.Unix()), Member: member})
	onlineCmd := pipe.ZCount(ctx, siteOnlineUsersZSet, strconv.FormatInt(cutoff, 10), "+inf")

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}

	return &app.HeartbeatResult{
		UV:     uvCountCmd.Val(),
		Online: onlineCmd.Val(),
	}, nil
}

// OnlineCount 读取当前在线人数（先移除窗口外成员再计数）。
func (s *SiteStatStore) OnlineCount(ctx context.Context) (int64, error) {
	if s.redis == nil {
		return 0, errRedisNotInitialized
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	now := time.Now()
	cutoff := now.Add(-time.Second * time.Duration(onlineWindowSeconds)).Unix()
	pipe := s.redis.Pipeline()
	pipe.ZRemRangeByScore(ctx, siteOnlineUsersZSet, "-inf", strconv.FormatInt(cutoff, 10))

	cmd := pipe.ZCount(ctx, siteOnlineUsersZSet, strconv.FormatInt(cutoff, 10), "+inf")
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}

	return cmd.Val(), nil
}

// DailyUV 读取指定自然日的访问 UV。
func (s *SiteStatStore) DailyUV(ctx context.Context, day time.Time) (int64, error) {
	if s.redis == nil {
		return 0, errRedisNotInitialized
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	key := siteDailyVisitUVPref + day.Format("20060102")

	return s.redis.SCard(ctx, key).Result()
}
