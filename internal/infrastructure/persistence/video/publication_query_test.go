package video_test

import (
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"
)

func TestAdminPublicationFieldsAndExplicitImmediateFilter(t *testing.T) {
	f := newRevisionFixture(t)
	if err := f.db.AutoMigrate(&po.User{}, &po.VideoResource{}, &po.Part{}, &po.VideoPartResource{}); err != nil {
		t.Fatal(err)
	}
	f.adminVideos = infra.NewAdminVideoQuery(f.db)
	f.detail = infra.NewDetailQuery(f.db)
	f.createUsers(t, po.User{Username: "alice"})
	planned := time.Now().Add(-time.Hour)
	approved := planned.Add(-time.Minute)
	scheduled := po.Video{Title: "scheduled", AuthorUsername: "alice", Status: enum.VideoStatusPublished, IsScheduled: true, PublishAt: &planned, ApprovedAt: &approved}
	immediate := po.Video{Title: "immediate", AuthorUsername: "alice", Status: enum.VideoStatusPublished}
	for _, row := range []*po.Video{&scheduled, &immediate} {
		if err := f.session().Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, filter := range []bool{true, false} {
		items, total, err := f.adminVideos.List(t.Context(), app.AdminVideoListParams{IsScheduled: &filter, Page: 1, PageSize: 10})
		if err != nil || total != 1 || len(items) != 1 || items[0].IsScheduled != filter {
			t.Fatalf("filter=%v, items=%+v, total=%d, err=%v", filter, items, total, err)
		}
		if filter && (items[0].PublishAt == nil || !items[0].PublishAt.Equal(planned) || items[0].ApprovedAt == nil || !items[0].ApprovedAt.Equal(approved)) {
			t.Fatalf("publication dates lost: %+v", items[0])
		}
		if !filter && (items[0].PublishAt != nil || items[0].ApprovedAt != nil) {
			t.Fatalf("immediate video acquired fake timestamps: %+v", items[0])
		}
	}
	meta, _, err := f.detail.LoadAny(t.Context(), scheduled.ID)
	if err != nil || !meta.IsScheduled || meta.PublishAt == nil || !meta.PublishAt.Equal(planned) || meta.ApprovedAt == nil || !meta.ApprovedAt.Equal(approved) {
		t.Fatalf("admin detail lost publication info: meta=%+v, err=%v", meta, err)
	}
}
