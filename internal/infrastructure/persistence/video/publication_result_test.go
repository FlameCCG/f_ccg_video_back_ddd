package video_test

import (
	"reflect"
	"testing"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	videodomain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

func newPublicationFixture(t *testing.T) *fixture {
	t.Helper()
	f := newRevisionFixture(t)
	if err := f.db.AutoMigrate(&po.VideoPart{}, &po.VideoPartResource{}, &po.VideoResource{}); err != nil {
		t.Fatal(err)
	}
	return f
}

func publicationDraft() *videodomain.PublishDraft {
	v, scheduledAt := videodomain.NewVideo(videodomain.VideoDraft{
		Title: "multipart", AuthorUsername: "alice", StorageType: enum.StorageTypeLocal,
	}, time.Now().Add(time.Hour), false, true, time.Now())
	return &videodomain.PublishDraft{
		Video: v, PublishAt: scheduledAt,
		Parts: []videodomain.VideoPart{{Title: "first", SortOrder: 0}, {Title: "second", SortOrder: 1}},
		PartResources: []*videodomain.ResourceDraft{
			{SourceFileName: "first.mp4", FileURL: "first.mp4"},
			{SourceFileName: "second.mp4", FileURL: "second.mp4"},
		},
	}
}

func TestPublicationFailureDoesNotAssignUnpersistedIdentities(t *testing.T) {
	f := newPublicationFixture(t)
	if err := f.db.Exec(`CREATE TRIGGER reject_initial_resource BEFORE INSERT ON video_part_resources BEGIN SELECT RAISE(ABORT, 'resource unavailable'); END`).Error; err != nil {
		t.Fatal(err)
	}
	draft := publicationDraft()
	beforeVideo, beforeSnapshot := draft.Video, draft.Video.Snapshot()
	beforeParts := append([]videodomain.VideoPart(nil), draft.Parts...)
	if result, err := f.videos.Persist(t.Context(), draft); err != videodomain.ErrCreatePartResourcesFailed || result != nil {
		t.Fatalf("resource failure returned a result or changed its error: result=%+v err=%v", result, err)
	}
	if draft.Video != beforeVideo || !reflect.DeepEqual(draft.Video.Snapshot(), beforeSnapshot) || !reflect.DeepEqual(draft.Parts, beforeParts) {
		t.Fatalf("failed transaction mutated input identities: parts=%+v", draft.Parts)
	}
	for _, model := range []any{&po.Video{}, &po.VideoPart{}, &po.VideoPartResource{}} {
		var count int64
		if err := f.db.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("failed transaction left %T rows: count=%d err=%v", model, count, err)
		}
	}
	if err := f.db.Exec("DROP TRIGGER reject_initial_resource").Error; err != nil {
		t.Fatal(err)
	}
	if result, err := f.videos.Persist(t.Context(), draft); err != nil || result.Video.ID() == 0 || len(result.Parts) != 2 {
		t.Fatalf("unchanged draft cannot be retried: result=%+v err=%v", result, err)
	}
}

func TestPublicationReturnsVideoAndPartsWithoutMutatingDraft(t *testing.T) {
	f := newPublicationFixture(t)
	draft := publicationDraft()
	beforeVideo, beforeSnapshot := draft.Video, draft.Video.Snapshot()
	beforeParts := append([]videodomain.VideoPart(nil), draft.Parts...)
	result, err := f.videos.Persist(t.Context(), draft)
	if err != nil {
		t.Fatal(err)
	}
	if result.Video.ID() == 0 || result.Video.CreatedAt().IsZero() || result.Video == draft.Video || len(result.Parts) != 2 {
		t.Fatalf("missing persisted result: %+v", result)
	}
	if draft.Video != beforeVideo || !reflect.DeepEqual(draft.Video.Snapshot(), beforeSnapshot) || !reflect.DeepEqual(draft.Parts, beforeParts) {
		t.Fatalf("successful create mutated its input: parts=%+v", draft.Parts)
	}
	var saved []po.VideoPart
	if err := f.db.Order("sort_order ASC").Find(&saved).Error; err != nil {
		t.Fatal(err)
	}
	for i, part := range result.Parts {
		if part.ID == 0 || part.ID != saved[i].ID || part.VideoID != result.Video.ID() || part.Title != beforeParts[i].Title || part.SortOrder != i {
			t.Fatalf("part identity/order lost: result=%+v stored=%+v", result.Parts, saved)
		}
		var resource po.VideoPartResource
		if err := f.db.Where("video_part_id = ?", part.ID).First(&resource).Error; err != nil || resource.FileURL != draft.PartResources[i].FileURL {
			t.Fatalf("resource belongs to wrong persisted part: part=%+v resource=%+v err=%v", part, resource, err)
		}
	}
	result.Parts[0].Title = "changed result"
	if !reflect.DeepEqual(draft.Parts, beforeParts) {
		t.Fatal("result shares the draft's mutable part slice")
	}
}
