package social_test

import (
	"errors"
	"testing"
	"time"

	socialdomain "f_ccg_video_back_ddd/internal/domain/social"
)

func TestEnsureFollowListVisible(t *testing.T) {
	t.Parallel()

	open := socialdomain.FollowPrivacy{Exists: true, OpenFollow: true, OpenFans: true}

	cases := []struct {
		name     string
		privacy  socialdomain.FollowPrivacy
		viewerID int
		ownerID  int
		page     int
		wantErr  error
	}{
		{name: "用户不存在", privacy: socialdomain.FollowPrivacy{}, viewerID: 1, ownerID: 2, page: 1, wantErr: socialdomain.ErrUserNotFound},
		{
			name:     "未公开关注",
			privacy:  socialdomain.FollowPrivacy{Exists: true, OpenFans: true},
			viewerID: 1,
			ownerID:  2,
			page:     1,
			wantErr:  socialdomain.ErrFollowNotPublic,
		},
		{name: "他人翻页超限", privacy: open, viewerID: 1, ownerID: 2, page: socialdomain.MaxForeignListPage + 1, wantErr: socialdomain.ErrForeignPageLimited},
		{name: "他人翻页边界可见", privacy: open, viewerID: 1, ownerID: 2, page: socialdomain.MaxForeignListPage},
		{
			name:     "本人绕过关闭的隐私开关和深翻页限制",
			privacy:  socialdomain.FollowPrivacy{Exists: true},
			viewerID: 2,
			ownerID:  2,
			page:     100,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := (socialdomain.PrivacyPolicy{}).EnsureFollowListVisible(tc.privacy, tc.viewerID, tc.ownerID, tc.page)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestEnsureFansListVisible(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		privacy  socialdomain.FollowPrivacy
		viewerID int
		ownerID  int
		page     int
		wantErr  error
	}{
		{name: "用户不存在", privacy: socialdomain.FollowPrivacy{}, viewerID: 1, ownerID: 2, page: 1, wantErr: socialdomain.ErrUserNotFound},
		{
			name:     "未公开粉丝",
			privacy:  socialdomain.FollowPrivacy{Exists: true, OpenFollow: true},
			viewerID: 1,
			ownerID:  2,
			page:     1,
			wantErr:  socialdomain.ErrFansNotPublic,
		},
		{
			name:     "他人翻页超限",
			privacy:  socialdomain.FollowPrivacy{Exists: true, OpenFans: true},
			viewerID: 1,
			ownerID:  2,
			page:     socialdomain.MaxForeignListPage + 1,
			wantErr:  socialdomain.ErrForeignPageLimited,
		},
		{
			name:     "本人绕过关闭的隐私开关和深翻页限制",
			privacy:  socialdomain.FollowPrivacy{Exists: true},
			viewerID: 9,
			ownerID:  9,
			page:     50,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := (socialdomain.PrivacyPolicy{}).EnsureFansListVisible(tc.privacy, tc.viewerID, tc.ownerID, tc.page)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestNewFocus(t *testing.T) {
	t.Parallel()

	rejected, err := socialdomain.NewFocus(7, 7)
	if !errors.Is(err, socialdomain.ErrCannotFocusSelf) || rejected != nil {
		t.Fatalf("self focus: focus=%v err=%v, want nil and %v", rejected, err, socialdomain.ErrCannotFocusSelf)
	}

	focus, err := socialdomain.NewFocus(7, 9)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if focus.UserID() != 7 || focus.FocusUserID() != 9 || focus.ID() != 0 || !focus.CreatedAt().IsZero() || !focus.UpdatedAt().IsZero() {
		t.Errorf("focus = %d -> %d id=%d, want 7 -> 9 without persistence fields", focus.UserID(), focus.FocusUserID(), focus.ID())
	}
}

func TestRestoreFocusKeepsHistoryAndSnapshotCopy(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	snapshot := socialdomain.FocusSnapshot{
		ID:          11,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		UserID:      4,
		FocusUserID: 4,
	}

	focus := socialdomain.RestoreFocus(snapshot)
	if focus.ID() != 11 || focus.UserID() != 4 || focus.FocusUserID() != 4 ||
		!focus.CreatedAt().Equal(createdAt) || !focus.UpdatedAt().Equal(updatedAt) {
		t.Fatalf("restored = id %d %d -> %d at %s/%s", focus.ID(), focus.UserID(), focus.FocusUserID(), focus.CreatedAt(), focus.UpdatedAt())
	}

	snapshot.ID = 99
	snapshot.UserID = 1
	snapshot.FocusUserID = 2
	snapshot.CreatedAt = time.Time{}
	snapshot.UpdatedAt = time.Time{}
	if focus.ID() != 11 || focus.UserID() != 4 || focus.FocusUserID() != 4 || !focus.CreatedAt().Equal(createdAt) {
		t.Fatal("修改入参快照不应改变已恢复的关注关系")
	}

	copied := focus.Snapshot()
	copied.ID = 8
	copied.UserID = 3
	copied.FocusUserID = 5
	copied.CreatedAt = time.Time{}
	copied.UpdatedAt = time.Time{}
	if focus.ID() != 11 || focus.UserID() != 4 || focus.FocusUserID() != 4 ||
		!focus.CreatedAt().Equal(createdAt) || !focus.UpdatedAt().Equal(updatedAt) {
		t.Fatal("快照副本不应与实体共享状态")
	}
}

func TestPrivacyDecisionLeavesInputUnchanged(t *testing.T) {
	t.Parallel()

	privacy := socialdomain.FollowPrivacy{Exists: true, OpenFans: true}
	before := privacy
	err := (socialdomain.PrivacyPolicy{}).EnsureFollowListVisible(privacy, 1, 2, 1)
	if !errors.Is(err, socialdomain.ErrFollowNotPublic) {
		t.Fatalf("err = %v, want %v", err, socialdomain.ErrFollowNotPublic)
	}

	if privacy != before {
		t.Fatalf("privacy = %+v, want unchanged %+v", privacy, before)
	}
}

func TestPrivacyPolicyEligibility(t *testing.T) {
	t.Parallel()

	policy := socialdomain.PrivacyPolicy{}

	if err := policy.EnsureUserID(0); !errors.Is(err, socialdomain.ErrUserIDRequired) {
		t.Fatalf("empty user: err = %v, want %v", err, socialdomain.ErrUserIDRequired)
	}

	if err := policy.EnsureUserID(3); err != nil {
		t.Fatalf("user id: unexpected err: %v", err)
	}

	if err := policy.EnsureTargetExists(false); !errors.Is(err, socialdomain.ErrTargetUserNotFound) {
		t.Fatalf("missing target: err = %v, want %v", err, socialdomain.ErrTargetUserNotFound)
	}

	if err := policy.EnsureTargetExists(true); err != nil {
		t.Fatalf("target exists: unexpected err: %v", err)
	}

	if err := policy.EnsureFocusRemoved(0); !errors.Is(err, socialdomain.ErrNotFocused) {
		t.Fatalf("not focused: err = %v, want %v", err, socialdomain.ErrNotFocused)
	}

	if err := policy.EnsureFocusRemoved(1); err != nil {
		t.Fatalf("removed: unexpected err: %v", err)
	}
}

func TestRelationIsMutual(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		relation socialdomain.Relation
		want     bool
	}{
		{name: "互相关注", relation: socialdomain.Relation{IsFocus: true, IsFans: true}, want: true},
		{name: "单向关注", relation: socialdomain.Relation{IsFocus: true}, want: false},
		{name: "单向被关注", relation: socialdomain.Relation{IsFans: true}, want: false},
		{name: "无关系", relation: socialdomain.Relation{}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.relation.IsMutual(); got != tc.want {
				t.Errorf("IsMutual() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEnsureRelationQueryable(t *testing.T) {
	t.Parallel()

	policy := socialdomain.PrivacyPolicy{}

	if err := policy.EnsureRelationQueryable(5, 5); !errors.Is(err, socialdomain.ErrSelfRelation) {
		t.Fatalf("err = %v, want %v", err, socialdomain.ErrSelfRelation)
	}

	if err := policy.EnsureRelationQueryable(5, 6); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}
