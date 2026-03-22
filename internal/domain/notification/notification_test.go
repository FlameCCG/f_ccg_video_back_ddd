package notification_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	notificationdomain "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

func TestNewNotificationStoresDraftWithoutExtraRules(t *testing.T) {
	draft := notificationdomain.Draft{}
	notice := notificationdomain.NewNotification(draft)
	draft.Title = "later"
	if notice == nil || notice.ID() != 0 || !notice.CreatedAt().IsZero() || !notice.UpdatedAt().IsZero() || notice.Type() != 0 || notice.ReceiverID() != 0 || notice.Title() != "" {
		t.Fatalf("empty draft must be stored as-is: %+v", notice.Snapshot())
	}

	system := notificationdomain.NewNotification(notificationdomain.Draft{
		Type:             enum.NotificationSystemType,
		ReceiverID:       notificationdomain.BroadcastReceiverID,
		ActionUserID:     2,
		ActionUserName:   "n",
		ActionUserAvatar: "a",
		Title:            "公告",
		Content:          "内容",
		Link:             "l",
		VideoID:          3,
		VideoTitle:       "v",
		DynamicID:        4,
		CommentID:        5,
	})
	snap := system.Snapshot()
	if snap.ID != 0 || !snap.CreatedAt.IsZero() || snap.Type != enum.NotificationSystemType || snap.ReceiverID != notificationdomain.BroadcastReceiverID || snap.Title != "公告" || snap.Content != "内容" || snap.CommentID != 5 || snap.DynamicID != 4 || snap.ActionUserID != 2 {
		t.Fatalf("creation dropped fields: %+v", snap)
	}
}

func TestRestoreNotificationKeepsHistoricalValues(t *testing.T) {
	created := time.Date(2019, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := created.Add(time.Minute)
	snapshot := notificationdomain.Snapshot{
		ID:           9,
		CreatedAt:    created,
		UpdatedAt:    updated,
		Type:         enum.Type(99),
		ReceiverID:   notificationdomain.BroadcastReceiverID,
		ActionUserID: 1,
		Title:        "",
		Content:      "old",
	}
	notice := notificationdomain.RestoreNotification(snapshot)
	snapshot.Title = "changed"
	if got := notice.Snapshot(); got.ID != 9 || !got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(updated) || got.Type != 99 || got.ReceiverID != notificationdomain.BroadcastReceiverID || got.Title != "" || got.Content != "old" || got.ActionUserID != 1 {
		t.Fatalf("restore rewrote history: %+v", got)
	}
}

func TestSnapshotCopiesNotificationState(t *testing.T) {
	created := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	notice := notificationdomain.RestoreNotification(notificationdomain.Snapshot{
		ID: 11, CreatedAt: created, Type: enum.NotificationAtType, ReceiverID: 6, Title: "t", Content: "c",
	})
	snap := notice.Snapshot()
	snap.Title = "mutated"
	snap.ReceiverID = 99
	if notice.Title() != "t" || notice.ReceiverID() != 6 {
		t.Fatal("snapshot shares notification state")
	}

	clone := notificationdomain.RestoreNotification(snap)
	clone.ReviseAsSystem(notificationdomain.SystemContent{Title: "other", ReceiverID: 1, Content: "x"})
	if notice.Title() != "t" || notice.Type() != enum.NotificationAtType || notice.Content() != "c" {
		t.Fatal("restored clone shares notification state")
	}
}

func TestSystemRevisionKeepsNotificationIdentityAndActor(t *testing.T) {
	created := time.Now()
	notice := notificationdomain.RestoreNotification(notificationdomain.Snapshot{
		ID: 7, CreatedAt: created, UpdatedAt: created, ActionUserID: 8, ActionUserName: "actor", ActionUserAvatar: "avatar",
		Type: enum.NotificationAtType, ReceiverID: 2, DynamicID: 3, CommentID: 4, Title: "old",
	})
	notice.ReviseAsSystem(notificationdomain.SystemContent{
		ReceiverID: notificationdomain.BroadcastReceiverID, Title: "公告", Content: "内容", Link: "https://example", VideoID: 9, VideoTitle: "视频",
	})
	if notice.Type() != enum.NotificationSystemType || notice.ReceiverID() != notificationdomain.BroadcastReceiverID || notice.Title() != "公告" || notice.Content() != "内容" || notice.Link() != "https://example" || notice.VideoID() != 9 || notice.VideoTitle() != "视频" {
		t.Fatalf("system fields must change together: %+v", notice.Snapshot())
	}
	if notice.ID() != 7 || notice.ActionUserID() != 8 || notice.ActionUserName() != "actor" || notice.ActionUserAvatar() != "avatar" || !notice.CreatedAt().Equal(created) || !notice.UpdatedAt().Equal(created) || notice.DynamicID() != 3 || notice.CommentID() != 4 {
		t.Fatalf("revision must preserve identity, actor and creation time: %+v", notice.Snapshot())
	}
}

func TestUndeletableRejectionKeepsNotificationState(t *testing.T) {
	system := notificationdomain.RestoreNotification(notificationdomain.Snapshot{
		ID: 4, Type: enum.NotificationSystemType, ReceiverID: 3, Title: "keep", Content: "body", ActionUserID: 8,
	})
	before := system.Snapshot()
	if err := system.EnsureDeletable(); !errors.Is(err, notificationdomain.ErrSystemNotificationNotDeletable) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(system.Snapshot(), before) {
		t.Fatalf("system state changed: %+v", system.Snapshot())
	}
	if system.DeletableBy(3) {
		t.Fatal("owner can delete a system notification")
	}

	broadcast := notificationdomain.RestoreNotification(notificationdomain.Snapshot{ID: 5, Type: enum.NotificationApplyType, ReceiverID: notificationdomain.BroadcastReceiverID, Title: "all"})
	before = broadcast.Snapshot()
	if err := broadcast.EnsureDeletable(); !errors.Is(err, notificationdomain.ErrSystemNotificationNotDeletable) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(broadcast.Snapshot(), before) || broadcast.DeletableBy(1) {
		t.Fatal("broadcast rejection changed state or became deletable")
	}

	personal := notificationdomain.RestoreNotification(notificationdomain.Snapshot{ID: 6, Type: enum.NotificationApplyType, ReceiverID: 5, Title: "mine"})
	if err := personal.EnsureDeletable(); err != nil {
		t.Fatal(err)
	}
	if !personal.DeletableBy(5) || personal.DeletableBy(6) || personal.DeletableBy(0) {
		t.Fatal("deletable receiver was wrong")
	}
}

func TestTypePolicyCategoriesMatchLegacyOrder(t *testing.T) {
	var policy notificationdomain.TypePolicy
	wantAll := []enum.Type{
		enum.NotificationApplyType,
		enum.NotificationVideoLikeType,
		enum.NotificationLikeCommentType,
		enum.NotificationLikeDynamicType,
		enum.NotificationAtType,
		enum.NotificationSystemType,
	}
	all := policy.AllTypes()
	if !reflect.DeepEqual(all, wantAll) {
		t.Fatalf("all=%v", all)
	}
	all[0] = 99
	if policy.AllTypes()[0] == 99 {
		t.Fatal("AllTypes shares its backing array")
	}
	if !reflect.DeepEqual(policy.TypesByCategory("reply"), []enum.Type{enum.NotificationApplyType}) ||
		!reflect.DeepEqual(policy.TypesByCategory("like"), []enum.Type{enum.NotificationVideoLikeType, enum.NotificationLikeCommentType, enum.NotificationLikeDynamicType}) ||
		!reflect.DeepEqual(policy.TypesByCategory("at"), []enum.Type{enum.NotificationAtType}) ||
		!reflect.DeepEqual(policy.TypesByCategory("system"), []enum.Type{enum.NotificationSystemType}) ||
		!reflect.DeepEqual(policy.TypesByCategory("unknown"), wantAll) {
		t.Fatal("category mapping changed")
	}
	grouped := map[enum.Type]int64{
		enum.NotificationApplyType:       9,
		enum.NotificationVideoLikeType:   1,
		enum.NotificationLikeCommentType: 2,
		enum.NotificationLikeDynamicType: 3,
		enum.NotificationAtType:          4,
		enum.NotificationSystemType:      5,
	}
	if policy.SumByCategory(grouped, "reply") != 9 || policy.SumByCategory(grouped, "like") != 6 || policy.SumByCategory(grouped, "at") != 4 || policy.SumByCategory(grouped, "system") != 5 {
		t.Fatal("category sums changed")
	}
}
