package identity

// AdminUserInfoResult 后台当前登录用户信息。
type AdminUserInfoResult struct {
	ID             uint            `json:"id"`
	Username       string          `json:"username"`
	Email          string          `json:"email"`
	Avatar         string          `json:"avatar"`
	Description    string          `json:"description"`
	Gender         int             `json:"gender"`
	Birthday       string          `json:"birthday"`
	Level          int             `json:"level"`
	Exp            int64           `json:"exp"`
	CoinCount      float64         `json:"coinCount"`
	FollowCount    int64           `json:"followCount"`
	FansCount      int64           `json:"fansCount"`
	DynamicCount   int64           `json:"dynamicCount"`
	CurrentRoleID  uint            `json:"currentRoleID"`
	RegisterSource string          `json:"registerSource"`
	RoleMap        map[uint]string `json:"roleMap"`
}
