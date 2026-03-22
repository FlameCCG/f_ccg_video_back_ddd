package dynamic

import "time"

// ReadEligibility 是本次浏览者、作者与关注关系的事实。
type ReadEligibility struct {
	ReaderID, AuthorID uint
	Following          bool
}

func (e ReadEligibility) ValidateLatest() error {
	if e.ReaderID == 0 {
		return ErrUserNotLogin
	}
	if e.AuthorID == 0 {
		return ErrAuthorIDRequired
	}
	return nil
}

func (e ReadEligibility) NeedsFollowing() bool { return e.ReaderID != e.AuthorID }
func (e ReadEligibility) AllowsLatest() bool   { return e.ReaderID == e.AuthorID || e.Following }

// 浏览视频的自动已读沿用仅关注对象的规则，不额外放行作者自己。
func (e ReadEligibility) AllowsVideo() bool { return e.Following }

type FollowOrderFacts struct {
	UserID       uint
	LatestWorkAt *time.Time
	FollowedAt   time.Time
}

type FollowOrder struct{}

func (FollowOrder) Less(a, b FollowOrderFacts) bool {
	aHas, bHas := a.LatestWorkAt != nil, b.LatestWorkAt != nil
	if aHas != bHas {
		return aHas
	}
	if aHas {
		if !a.LatestWorkAt.Equal(*b.LatestWorkAt) {
			return a.LatestWorkAt.After(*b.LatestWorkAt)
		}
	} else if !a.FollowedAt.Equal(b.FollowedAt) {
		return a.FollowedAt.After(b.FollowedAt)
	}
	return a.UserID > b.UserID
}
