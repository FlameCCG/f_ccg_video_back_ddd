package comment

import (
	"reflect"
	"testing"
)

func TestCommentNoticeSelectionPreservesRecipientAndReplyPrecedence(t *testing.T) {
	policy := NoticePolicy{}
	if policy.VideoAuthor(1, 1) || policy.DynamicAuthor(1, 1) || policy.Reply(1, 1) {
		t.Fatal("self notification allowed")
	}
	if !policy.VideoAuthor(1, 0) || policy.DynamicAuthor(1, 0) || policy.Reply(1, 0) {
		t.Fatal("historical zero ID handling changed")
	}
	if got := policy.LastByReceiver([]uint{7, 8, 7}); !reflect.DeepEqual(got, []int{2, 1}) {
		t.Fatal(got)
	}
	if got := policy.AtRecipients([]uint{0, 7, 8, 7}); !reflect.DeepEqual(got, []uint{7, 8}) {
		t.Fatal(got)
	}
	if RestoreComment(Snapshot{UserID: 1}).ShouldNotifyLike(1) || !RestoreComment(Snapshot{UserID: 2}).ShouldNotifyLike(1) {
		t.Fatal("like recipient changed")
	}
}
