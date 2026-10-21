package video

// 投币与收藏规则（与原 service/video_service 的常量与判定一致）。
const (
	// MaxCoinsPerVideo 每人每视频最多投币数（无法撤销）.
	MaxCoinsPerVideo int64 = 2
	// CoinAuthorRewardRate 作者获得的投币奖励比例（投币数的 10%）.
	CoinAuthorRewardRate = 0.1
	// FavoriteAuthorReward 作品被收藏时作者获得的硬币奖励.
	FavoriteAuthorReward = 0.1
)

// ClampCoins 裁剪单次投币数量到 [1, MaxCoinsPerVideo]。
// coins <= 0 时返回 ErrCoinCountInvalid（与原 applyCoinInTx 一致）。
func ClampCoins(coins int64) (int64, error) {
	add := min(coins, MaxCoinsPerVideo)
	if add <= 0 {
		return 0, ErrCoinCountInvalid
	}

	return add, nil
}

// NormalizeTripleCoins 归一化三连的投币数量：不传或非法时按 1，超过上限截断为 2。
// 与原 InteractionService.Triple 的入口裁剪逐行一致。
func NormalizeTripleCoins(coins int64) int64 {
	if coins <= 0 {
		coins = 1
	}

	if coins > MaxCoinsPerVideo {
		coins = MaxCoinsPerVideo
	}

	return coins
}

// CoinAuthorReward 计算作者应获得的投币奖励（未做精度取整，由应用层用 coinutil.Round 处理）。
func CoinAuthorReward(coins int64) float64 {
	if coins <= 0 {
		return 0
	}

	return float64(coins) * CoinAuthorRewardRate
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

// 用户视频列表排序（与原 user_video_service.go 常量值一致）。
const (
	// UserVideoSortLatest 按最新排序.
	UserVideoSortLatest = 0
	// UserVideoSortViews 按播放量排序.
	UserVideoSortViews = 1
	// UserVideoSortFavorite 按收藏排序.
	UserVideoSortFavorite = 2
)

// 用户视频审核态（与原 user_video_service.go 常量值一致）。
const (
	// UserVideoAuditPublished 已发布审核态.
	UserVideoAuditPublished = 1
	// UserVideoAuditPrivate 私密审核态.
	UserVideoAuditPrivate = 2
	// UserVideoAuditReviewing 审核中.
	UserVideoAuditReviewing = 4
	// UserVideoAuditScheduled 定时待发布.
	UserVideoAuditScheduled = 5
)

// 最近互动动作（与原 user_video_service.go 常量一致）。
const (
	// ActionLike 点赞动作.
	ActionLike = "like"
	// ActionCoin 投币动作.
	ActionCoin = "coin"
)

// 收藏夹内视频排序（与原 folder_service.go 常量值一致）。
const (
	// FolderVideoSortLatest 按最新收藏排序.
	FolderVideoSortLatest = iota
	// FolderVideoSortViews 按播放量排序.
	FolderVideoSortViews
	// FolderVideoSortLikes 按点赞排序.
	FolderVideoSortLikes
)

// ValidateRecentActionType 校验最近互动动作类型。
func ValidateRecentActionType(action string) error {
	switch action {
	case ActionLike, ActionCoin:
		return nil
	default:
		return ErrActionUnsupported
	}
}
