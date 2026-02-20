package social_test

import (
	"errors"
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/social"
)

func TestEnsureFollowListVisible(t *testing.T) {
	t.Parallel()

	open := domain.FollowPrivacy{Exists: true, OpenFollow: true, OpenFans: true}

	cases := []struct {
		name     string
		privacy  domain.FollowPrivacy
		viewerID int
		ownerID  int
		page     int
		wantErr  error
	}{
		{name: "用户不存在", privacy: domain.FollowPrivacy{}, viewerID: 1, ownerID: 2, page: 1, wantErr: domain.ErrUserNotFound},
		{
			name:     "未公开关注",
			privacy:  domain.FollowPrivacy{Exists: true, OpenFans: true},
			viewerID: 1,
			ownerID:  2,
			page:     1,
			wantErr:  domain.ErrFollowNotPublic,
		},
		{name: "他人翻页超限", privacy: open, viewerID: 1, ownerID: 2, page: domain.MaxForeignListPage + 1, wantErr: domain.ErrForeignPageLimited},
		{name: "他人翻页边界可见", privacy: open, viewerID: 1, ownerID: 2, page: domain.MaxForeignListPage},
		{
			name:     "本人绕过关闭的隐私开关和深翻页限制",
			privacy:  domain.FollowPrivacy{Exists: true},
			viewerID: 2,
			ownerID:  2,
			page:     100,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := domain.EnsureFollowListVisible(tc.privacy, tc.viewerID, tc.ownerID, tc.page)
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
		privacy  domain.FollowPrivacy
		viewerID int
		ownerID  int
		page     int
		wantErr  error
	}{
		{name: "用户不存在", privacy: domain.FollowPrivacy{}, viewerID: 1, ownerID: 2, page: 1, wantErr: domain.ErrUserNotFound},
		{
			name:     "未公开粉丝",
			privacy:  domain.FollowPrivacy{Exists: true, OpenFollow: true},
			viewerID: 1,
			ownerID:  2,
			page:     1,
			wantErr:  domain.ErrFansNotPublic,
		},
		{
			name:     "他人翻页超限",
			privacy:  domain.FollowPrivacy{Exists: true, OpenFans: true},
			viewerID: 1,
			ownerID:  2,
			page:     domain.MaxForeignListPage + 1,
			wantErr:  domain.ErrForeignPageLimited,
		},
		{
			name:     "本人绕过关闭的隐私开关和深翻页限制",
			privacy:  domain.FollowPrivacy{Exists: true},
			viewerID: 9,
			ownerID:  9,
			page:     50,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := domain.EnsureFansListVisible(tc.privacy, tc.viewerID, tc.ownerID, tc.page)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestNewFocus(t *testing.T) {
	t.Parallel()

	if _, err := domain.NewFocus(7, 7); !errors.Is(err, domain.ErrCannotFocusSelf) {
		t.Fatalf("self focus: err = %v, want %v", err, domain.ErrCannotFocusSelf)
	}

	focus, err := domain.NewFocus(7, 9)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if focus.UserID != 7 || focus.FocusUserID != 9 {
		t.Errorf("focus = %+v, want 7 -> 9", focus)
	}
}

func TestRelationIsMutual(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		relation domain.Relation
		want     bool
	}{
		{name: "互相关注", relation: domain.Relation{IsFocus: true, IsFans: true}, want: true},
		{name: "单向关注", relation: domain.Relation{IsFocus: true}, want: false},
		{name: "单向被关注", relation: domain.Relation{IsFans: true}, want: false},
		{name: "无关系", relation: domain.Relation{}, want: false},
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

	if err := domain.EnsureRelationQueryable(5, 5); !errors.Is(err, domain.ErrSelfRelation) {
		t.Fatalf("err = %v, want %v", err, domain.ErrSelfRelation)
	}

	if err := domain.EnsureRelationQueryable(5, 6); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}
