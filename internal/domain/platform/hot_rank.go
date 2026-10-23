// Package platform 表达热门榜衰减、到期解封和评论点赞意图折叠。
//
// hot_rank.go、ban_release.go、comment_like.go 只根据输入事实计算结果。
// cron 表达式、Redis 队列和 SQL 批量写回由外层负责；把时间和待处理数据显式传入可重现边界条件。
//
// 本包保持纯 Go 规则与端口，不依赖 HTTP、数据库、缓存、消息框架或配置加载器。
// 领域函数尽量显式接收事实与时间，让规则可以脱离运行环境阅读和验证。
package platform

// 热门榜衰减参数（与原 service/cron_service/decay_video_hot.go 完全一致）。
const (
	// HotScoreDecayFactor 每轮衰减保留的分值比例。
	HotScoreDecayFactor = 0.97
	// HotScoreClearThreshold 分值衰减到该阈值及以下时直接移出榜单。
	HotScoreClearThreshold = 0.01
	// HotRankMaxSize 榜单保留的最大成员数，超出部分按分值从低到高裁剪。
	HotRankMaxSize = 200
)

// HotRankEntry 热门榜单中的一个成员及其当前分值。
type HotRankEntry struct {
	// Member 成员标识（视频 ID 的字符串形式）。
	Member string
	// Score 当前分值。
	Score float64
}

// HotRankUpdate 单个成员的衰减结果：写回新分值或移出榜单。
type HotRankUpdate struct {
	// Member 成员标识。
	Member string
	// Score 衰减后的新分值；Remove 为真时无意义。
	Score float64
	// Remove 为真表示该成员应被移出榜单。
	Remove bool
}

// HotRankDecayPolicy 热门榜时间衰减规则，避免老视频长期占榜。
type HotRankDecayPolicy struct {
	// Factor 每轮保留的分值比例。
	Factor float64
	// ClearThreshold 低于或等于该分值即移出榜单。
	ClearThreshold float64
	// MaxSize 榜单容量上限。
	MaxSize int64
}

// NewHotRankDecayPolicy 构造默认衰减规则（参数与原实现常量一致）。
func NewHotRankDecayPolicy() HotRankDecayPolicy {
	return HotRankDecayPolicy{
		Factor:         HotScoreDecayFactor,
		ClearThreshold: HotScoreClearThreshold,
		MaxSize:        HotRankMaxSize,
	}
}

// Decay 返回衰减后的分值，以及该成员是否应被移出榜单。
func (p HotRankDecayPolicy) Decay(score float64) (float64, bool) {
	decayed := score * p.Factor

	return decayed, decayed <= p.ClearThreshold
}

// DecayAll 按入参顺序对榜单成员逐个应用衰减，返回同序的写回指令。
// 保持顺序是为了让基础设施侧的 Redis 管线操作次序与原实现逐条一致。
func (p HotRankDecayPolicy) DecayAll(entries []HotRankEntry) []HotRankUpdate {
	if len(entries) == 0 {
		return nil
	}

	updates := make([]HotRankUpdate, 0, len(entries))

	for _, entry := range entries {
		score, remove := p.Decay(entry.Score)
		updates = append(updates, HotRankUpdate{
			Member: entry.Member,
			Score:  score,
			Remove: remove,
		})
	}

	return updates
}

// OverflowCount 返回超出榜单容量、需要裁剪的成员数量；未超出时返回 0。
func (p HotRankDecayPolicy) OverflowCount(total int64) int64 {
	if total > p.MaxSize {
		return total - p.MaxSize
	}

	return 0
}
