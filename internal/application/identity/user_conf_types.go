package identity

// UpdateUserConfInput 更新用户配置入参。
type UpdateUserConfInput struct {
	UserID        uint
	OpenCollect   *bool
	OpenFans      *bool
	OpenFollow    *bool
	OpenLikeVideo *bool
	OpenCoinVideo *bool
	BannerID      *uint
	BannerURL     string
	LikeTags      *[]string
}
