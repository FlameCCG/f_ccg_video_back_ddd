package identity

// UserConf 用户配置（用户聚合内的实体，一对一挂在 User 下）。
//
// LikeTags 为已解析后的标签集合：持久化层负责 JSON 编解码，
// 解析失败时返回空切片（与原 resolveUserLikeTags 行为一致）。
type UserConf struct {
	UserID        uint
	LikeTags      []string
	BannerID      uint
	BannerURL     string
	OpenCollect   bool
	OpenFans      bool
	OpenFollow    bool
	OpenLikeVideo bool
	OpenCoinVideo bool
}

// UserConfChanges 用户配置的部分更新集合；nil 字段表示本次不更新。
//
// 与原 UpdateUserConfView 中手工拼装的 updates map 一一对应，
// 但把「哪些列可写」的知识从接口层收敛到领域层。
type UserConfChanges struct {
	OpenCollect   *bool
	OpenFans      *bool
	OpenFollow    *bool
	OpenLikeVideo *bool
	OpenCoinVideo *bool
	BannerID      *uint
	BannerURL     *string
	LikeTags      *[]string
}

// IsEmpty 报告是否没有任何字段需要更新。
func (c UserConfChanges) IsEmpty() bool {
	return c.OpenCollect == nil &&
		c.OpenFans == nil &&
		c.OpenFollow == nil &&
		c.OpenLikeVideo == nil &&
		c.OpenCoinVideo == nil &&
		c.BannerID == nil &&
		c.BannerURL == nil &&
		c.LikeTags == nil
}
