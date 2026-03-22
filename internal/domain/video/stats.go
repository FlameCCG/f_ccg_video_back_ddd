package video

// Stats 视频数字统计值对象。
//
// Redis stats payload 按此结构序列化，JSON 字段属于外部契约。
type Stats struct {
	Views         int64 `json:"views"`
	Likes         int64 `json:"likes"`
	CommentCount  int64 `json:"commentCount"`
	CoinCount     int64 `json:"coinCount"`
	FavoriteCount int64 `json:"favoriteCount"`
	DanmuCount    int64 `json:"danmuCount"`
}

// StatsDelta 视频统计增量。
type StatsDelta struct {
	Views         int64
	Likes         int64
	CommentCount  int64
	CoinCount     int64
	FavoriteCount int64
	DanmuCount    int64
}

// IsZero 判断增量是否为空（无需任何写入）。
func (d StatsDelta) IsZero() bool {
	return d.Views == 0 &&
		d.Likes == 0 &&
		d.CommentCount == 0 &&
		d.CoinCount == 0 &&
		d.FavoriteCount == 0 &&
		d.DanmuCount == 0
}

// UserFlags 当前用户对视频的互动标记。
type UserFlags struct {
	IsLiked     bool
	IsFavorited bool
	IsCoined    bool
}
