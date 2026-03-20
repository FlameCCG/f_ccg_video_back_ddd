package video

// LikeResult 点赞结果。
type LikeResult struct {
	Likes int64 `json:"likes"`
}

// CoinResult 投币结果。
type CoinResult struct {
	CoinCount int64 `json:"coinCount"`
}

// FavoriteResult 收藏结果。
type FavoriteResult struct {
	FavoriteCount int64 `json:"favoriteCount"`
}

// ViewResult 播放量结果。
type ViewResult struct {
	Views int64 `json:"views"`
}

// TripleResult 一键三连结果。
type TripleResult struct {
	Likes         int64 `json:"likes"`
	CoinCount     int64 `json:"coinCount"`
	FavoriteCount int64 `json:"favoriteCount"`
	IsLiked       bool  `json:"isLiked"`
	IsFavorited   bool  `json:"isFavorited"`
	IsCoined      bool  `json:"isCoined"`
}
