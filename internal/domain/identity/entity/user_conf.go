package entity

// UserConf 用户配置（用户聚合内的实体，一对一挂在 User 下）。
//
// LikeTags 为已解析后的标签集合：持久化层负责 JSON 编解码，
// 解析失败时返回空切片。
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

// UserConfChanges 表示用户配置中允许更新的字段；nil 表示不更新。
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
