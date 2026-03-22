package dynamic

import (
	"errors"
	"testing"
	"time"
)

func TestReadEligibilityPreservesManualAndAutomaticReadDifferences(t *testing.T) {
	self := ReadEligibility{ReaderID: 1, AuthorID: 1}
	if self.ValidateLatest() != nil || self.NeedsFollowing() || !self.AllowsLatest() || self.AllowsVideo() {
		t.Fatal("self read eligibility changed")
	}
	other := ReadEligibility{ReaderID: 1, AuthorID: 2}
	if !other.NeedsFollowing() || other.AllowsLatest() || other.AllowsVideo() {
		t.Fatal("unfollowed author became eligible")
	}
	other.Following = true
	if !other.AllowsLatest() || !other.AllowsVideo() {
		t.Fatal("followed author is ineligible")
	}
	if !errors.Is((ReadEligibility{}).ValidateLatest(), ErrUserNotLogin) || !errors.Is((ReadEligibility{ReaderID: 1}).ValidateLatest(), ErrAuthorIDRequired) {
		t.Fatal("validation order changed")
	}
}

func TestFollowOrderUsesActivityThenFollowTimeThenIdentity(t *testing.T) {
	now := time.Now()
	before := now.Add(-time.Hour)
	order := FollowOrder{}
	if !order.Less(FollowOrderFacts{UserID: 1, LatestWorkAt: &before}, FollowOrderFacts{UserID: 9, FollowedAt: now}) ||
		!order.Less(FollowOrderFacts{LatestWorkAt: &now}, FollowOrderFacts{LatestWorkAt: &before}) ||
		!order.Less(FollowOrderFacts{FollowedAt: now}, FollowOrderFacts{FollowedAt: before}) ||
		!order.Less(FollowOrderFacts{UserID: 9}, FollowOrderFacts{UserID: 1}) {
		t.Fatal("follow ordering changed")
	}
}
