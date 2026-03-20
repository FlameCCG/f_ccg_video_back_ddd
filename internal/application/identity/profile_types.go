package identity

// UserInfoResult 当前登录用户信息。
type UserInfoResult struct {
	IsAdmin        bool     `json:"isAdmin"`
	ID             uint     `json:"id"`
	Username       string   `json:"username"`
	Email          string   `json:"email"`
	Avatar         string   `json:"avatar"`
	BannerURL      string   `json:"bannerUrl"`
	Description    string   `json:"description"`
	Gender         int      `json:"gender"`
	Birthday       string   `json:"birthday"`
	Level          int      `json:"level"`
	Exp            int64    `json:"exp"`
	CoinCount      float64  `json:"coinCount"`
	FollowCount    int64    `json:"followCount"`
	FansCount      int64    `json:"fansCount"`
	DynamicCount   int64    `json:"dynamicCount"`
	RegisterSource string   `json:"registerSource"`
	LikeTags       []string `json:"likeTags"`
	// HasPassword 是否已设置登录密码；第三方首次登录用户可能为 false。
	// 前端「修改密码」可据此决定是否要求输入旧密码。
	HasPassword bool `json:"hasPassword"`
	// IsThirdParty 是否绑定了第三方登录（OpenID 非空）。
	IsThirdParty bool `json:"isThirdParty"`
}

// UserDetailResult 用户公开详情。
type UserDetailResult struct {
	ID          uint     `json:"id"`
	Username    string   `json:"username"`
	Avatar      string   `json:"avatar"`
	BannerURL   string   `json:"bannerUrl"`
	Description string   `json:"description"`
	Gender      int      `json:"gender"`
	Birthday    string   `json:"birthday"`
	Level       int      `json:"level"`
	Exp         int64    `json:"exp"`
	FollowCount int64    `json:"followCount"`
	FansCount   int64    `json:"fansCount"`
	TotalLikes  int64    `json:"totalLikes"`
	TotalViews  int64    `json:"totalViews"`
	LikeTags    []string `json:"likeTags"`
}

// UpdateProfileInput 更新用户资料入参。
type UpdateProfileInput struct {
	// UserID 待更新用户；用户侧取当前登录用户，管理端取请求参数。
	UserID uint
	// Username 空串表示不更新。
	Username string
	// Avatar 空串表示不更新。
	Avatar string
	// Description 空串表示不更新。
	Description string
	// Gender nil 表示不更新。
	Gender *int
	// Birthday 空串表示不更新，否则必须是 2006-01-02。
	Birthday string
}
