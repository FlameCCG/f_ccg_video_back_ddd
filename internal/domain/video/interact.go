package video

import "f_ccg_video_back_ddd/pkg/coinutil"

// 投币与收藏规则。
const (
	// MaxCoinsPerVideo 每人每视频最多投币数（无法撤销）.
	MaxCoinsPerVideo int64 = 2
	// CoinAuthorRewardRate 作者获得的投币奖励比例（投币数的 10%）.
	CoinAuthorRewardRate = 0.1
	// FavoriteAuthorReward 作品被收藏时作者获得的硬币奖励.
	FavoriteAuthorReward = 0.1
)

// FavoritePolicy 裁定首次收藏及作者奖励资格，普通收藏和三连采用同一规则。
type FavoritePolicy struct{}

// FirstAddition 只有新增关联且此前未收藏该作品时才增加作品收藏数。
func (FavoritePolicy) FirstAddition(linkCreated, alreadyFavorited bool) bool {
	return linkCreated && !alreadyFavorited
}

// AuthorReward 自己收藏自己的作品或作者缺失时不发奖励。
// 同一用户与作品的终身去重由应用层协调流水仓储完成。
func (FavoritePolicy) AuthorReward(authorID, actorID uint) float64 {
	if authorID == 0 || authorID == actorID {
		return 0
	}
	return FavoriteAuthorReward
}

// CoinPolicy 裁剪投币数量并计算作者奖励。
type CoinPolicy struct{}

// Clamp 裁剪单次投币数量到 [1, MaxCoinsPerVideo]。
// coins <= 0 时返回 ErrCoinCountInvalid。
func (CoinPolicy) Clamp(coins int64) (int64, error) {
	add := min(coins, MaxCoinsPerVideo)
	if add <= 0 {
		return 0, ErrCoinCountInvalid
	}

	return add, nil
}

// Triple 归一化三连的投币数量：不传或非法时按 1，超过上限截断为 2。。
func (CoinPolicy) Triple(coins int64) int64 {
	if coins <= 0 {
		coins = 1
	}

	if coins > MaxCoinsPerVideo {
		coins = MaxCoinsPerVideo
	}

	return coins
}

// AuthorReward 计算作者投币奖励，按既有硬币精度返回记账金额。
func (CoinPolicy) AuthorReward(coins int64) float64 {
	if coins <= 0 {
		return 0
	}

	return coinutil.Round(float64(coins) * CoinAuthorRewardRate)
}

// TripleOutcome 三连在一个事务内实际产生的增量与副作用标记。
type TripleOutcome struct {
	LikeDelta   int64
	FavDelta    int64
	CoinDelta   int64
	AuthorID    uint
	NewlyLiked  bool
	NewlyFaved  bool
	NewlyCoined bool
}

// 最近互动动作。
const (
	// ActionLike 点赞动作.
	ActionLike = "like"
	// ActionCoin 投币动作.
	ActionCoin = "coin"
)

// 收藏夹内视频排序。
const (
	// FolderVideoSortLatest 按最新收藏排序.
	FolderVideoSortLatest = iota
	// FolderVideoSortViews 按播放量排序.
	FolderVideoSortViews
	// FolderVideoSortLikes 按点赞排序.
	FolderVideoSortLikes
)
