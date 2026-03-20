package search

// MentionUser 用于评论 @ 联想.
type MentionUser struct {
	ID            uint   `json:"id"`
	Username      string `json:"username"`
	Avatar        string `json:"avatar"`
	FollowerCount int64  `json:"followerCount"`
	IsFriend      bool   `json:"isFriend"`
}
