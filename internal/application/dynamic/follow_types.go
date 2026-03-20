package dynamic

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// FollowUserItem 关注列表条目（读模型）.
type FollowUserItem struct {
	UserID         uint          `json:"userId"`
	Username       string        `json:"username"`
	Avatar         string        `json:"avatar"`
	FollowedAt     time.Time     `json:"followedAt"`
	LatestWorkAt   *time.Time    `json:"latestWorkAt,omitempty"`
	LatestWorkType enum.WorkType `json:"latestWorkType,omitempty"`
	IsUnread       bool          `json:"isUnread"`
}

// FollowRelation 当前用户的一条关注关系（含被关注者资料）。
type FollowRelation struct {
	FocusUserID uint
	FollowedAt  time.Time
	Username    string
	Avatar      string
}
