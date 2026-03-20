package comment

// UserProfile 用户投影（id / username / avatar）。
type UserProfile struct {
	ID       uint
	Username string
	Avatar   string
}

// VideoBrief 视频投影。
//
// 不同查询方法填充的字段不同：PublishedVideoBrief 填 ID/Title/AuthorUsername，
// VideoBrief 只填 ID/Title。
type VideoBrief struct {
	ID             uint
	Title          string
	AuthorUsername string
}

// VideoAuthorRef 视频作者引用投影（用于删除/置顶鉴权）。
type VideoAuthorRef struct {
	VideoID uint
	// AuthorUserID 为作者用户 ID；作者用户记录缺失时为 0。
	AuthorUserID uint
}

// DynamicBrief 动态投影（id / 发布者 ID）。
type DynamicBrief struct {
	ID     uint
	UserID uint
}
