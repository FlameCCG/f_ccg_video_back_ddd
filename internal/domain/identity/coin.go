package identity

// 硬币奖励规则。
const (
	// CoinLoginReward 每日登录硬币奖励.
	CoinLoginReward float64 = 1.0
	// CoinFavoriteReward 作品被收藏时作者获得的硬币奖励.
	CoinFavoriteReward float64 = 0.1
)

// FavoriteRewardPolicy 保护身份账本的收藏奖励追回资格。
// 是否已发放、是否已追回由应用层在账户锁内取得流水事实。
type FavoriteRewardPolicy struct{}

func (FavoriteRewardPolicy) CanRevoke(authorID, actorID uint) bool {
	return authorID != 0 && authorID != actorID
}
