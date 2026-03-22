package identity_test

import (
	"errors"
	"reflect"
	"testing"

	identity "f_ccg_video_back_ddd/internal/domain/identity"
)

func TestBannerEditsPreserveHistoryAndRejectInvalidChanges(t *testing.T) {
	p := identity.BannerPolicy{}
	original := []uint{0, 2, 2}
	change, err := p.ReplaceRegisterDefault(0, original, 3)
	if err != nil || change.PreviousID != 19 || change.RegisterID != 3 || !reflect.DeepEqual(change.Defaults, []uint{0, 2, 2, 19}) {
		t.Fatal("replacement changed legacy list semantics", change, err)
	}
	change.Defaults[0] = 99
	if original[0] != 0 {
		t.Fatal("replacement aliases original list")
	}
	if _, err := p.ReplaceRegisterDefault(0, original, 19); !errors.Is(err, identity.ErrBannerAlreadyRegisterDefault) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		ids    []uint
		action int
		want   []uint
		err    error
	}{
		{[]uint{3}, identity.UserBannerActionAdd, []uint{2, 3}, nil},
		{[]uint{2}, identity.UserBannerActionAdd, nil, identity.ErrBannerAlreadyInDefaults},
		{[]uint{2}, identity.UserBannerActionDelete, []uint{}, nil},
		{[]uint{19}, 99, nil, identity.ErrRegisterDefaultBannerLocked},
		{[]uint{3}, 99, nil, identity.ErrBannerActionInvalid},
	} {
		got, err := p.EditDefaults(original, 19, tc.ids, tc.action)
		if !errors.Is(err, tc.err) || !reflect.DeepEqual(got, tc.want) || !reflect.DeepEqual(original, []uint{0, 2, 2}) {
			t.Fatal("list edit violated policy or changed source", got, err)
		}
	}
}

func TestRecentActionPrivacyKeepsOwnerAndActionDistinctions(t *testing.T) {
	if err := (identity.ActionVisibility{}).EnsureVisible(1, 1, "like"); err != nil {
		t.Fatal("owner view must not require a privacy record", err)
	}
	if err := (identity.ActionVisibility{}).EnsureVisible(1, 2, "like"); !errors.Is(err, identity.ErrUserNotFound) {
		t.Fatal(err)
	}
	v := identity.ActionVisibility{Exists: true, OpenLikeVideo: true}
	if err := v.EnsureVisible(1, 2, "like"); err != nil {
		t.Fatal(err)
	}
	if err := v.EnsureVisible(1, 2, "coin"); !errors.Is(err, identity.ErrRecentCoinedNotPublic) {
		t.Fatal(err)
	}
	v.OpenLikeVideo = false
	if err := v.EnsureVisible(1, 2, "like"); !errors.Is(err, identity.ErrRecentLikedNotPublic) {
		t.Fatal(err)
	}
}
