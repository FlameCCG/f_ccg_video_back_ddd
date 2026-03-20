package entity_test

import (
	"testing"
	"time"

	"f_ccg_video_back_ddd/internal/domain/notification/entity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

func TestSystemRevisionKeepsNotificationIdentityAndActor(t *testing.T) {
	created := time.Now()
	notice := &entity.Notification{ID: 7, CreatedAt: created, ActionUserID: 8, Type: enum.NotificationAtType}
	notice.ReviseAsSystem(entity.SystemContent{ReceiverID: entity.BroadcastReceiverID, Title: "公告", Content: "内容", VideoID: 9})
	if notice.Type != enum.NotificationSystemType || notice.ReceiverID != entity.BroadcastReceiverID || notice.Title != "公告" || notice.VideoID != 9 {
		t.Fatalf("system fields must change together: %+v", notice)
	}
	if notice.ID != 7 || notice.ActionUserID != 8 || !notice.CreatedAt.Equal(created) {
		t.Fatalf("revision must preserve identity, actor and creation time: %+v", notice)
	}
}
