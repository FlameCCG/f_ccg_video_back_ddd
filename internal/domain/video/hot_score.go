package video

// 热度分权重（与原 service/video_service/hot_score.go 完全一致）。
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

// HotScoreDelta 计算一次互动应写入热度 ZSet 的分数增量。
// weight 或 delta 为 0 时返回 0，调用方应跳过写入。
func HotScoreDelta(weight float64, delta int64) float64 {
	if weight == 0 || delta == 0 {
		return 0
	}

	return float64(delta) * weight
}

// RankScore 计算排行榜综合分。
//
// 与原 computeRankScore 的权重与优先级一致：stats（Redis 权威计数）优先，
// 缺失时回落视频行内计数。
func RankScore(row Stats, cached *Stats) float64 {
	src := row
	if cached != nil {
		src = *cached
	}

	return float64(src.Views) +
		float64(src.Likes)*HotScoreLikeWeight +
		float64(src.CoinCount)*HotScoreCoinWeight +
		float64(src.FavoriteCount)*HotScoreFavoriteWeight +
		float64(src.CommentCount)*HotScoreCommentWeight +
		float64(src.DanmuCount)*HotScoreDanmuWeight
}
