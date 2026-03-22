package video

// 热度分权重。
const (
	// HotScoreViewWeight 播放权重.
	HotScoreViewWeight = 1.0
	// HotScoreLikeWeight 点赞权重.
	HotScoreLikeWeight = 3.0
	// HotScoreCoinWeight 投币权重.
	HotScoreCoinWeight = 4.0
	// HotScoreFavoriteWeight 收藏权重.
	HotScoreFavoriteWeight = 4.0
	// HotScoreCommentWeight 评论权重.
	HotScoreCommentWeight = 2.0
	// HotScoreDanmuWeight 弹幕权重.
	HotScoreDanmuWeight = 1.0
)

// HeatPolicy 计算热度 ZSet 的分数增量。
type HeatPolicy struct{}

// Delta 计算一次互动应写入热度 ZSet 的分数增量。
// weight 或 delta 为 0 时返回 0，调用方应跳过写入。
func (HeatPolicy) Delta(weight float64, delta int64) float64 {
	if weight == 0 || delta == 0 {
		return 0
	}

	return float64(delta) * weight
}
