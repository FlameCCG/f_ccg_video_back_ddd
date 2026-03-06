package video_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"

	"go.uber.org/zap"
)

func scheduleVideo(t *testing.T, f *fixture, status enum.VideoStatus) uint {
	t.Helper()
	row := po.Video{Title: "old", AuthorUsername: "alice", Status: status}
	if err := f.session().Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func scheduleRunner(f *fixture, store app.ScheduleStore, review bool) *app.Scheduler {
	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{ContentReview: &site.ContentReview{Enable: review}}})
	return app.NewScheduler(gormx.NewTxManager(f.db), f.videos, store, app.NewVectorSyncService(nil, nil, nil), provider, zap.NewNop())
}

func requireScheduleStatus(t *testing.T, f *fixture, id uint, want enum.VideoStatus) {
	t.Helper()
	video, err := f.videos.FindByID(t.Context(), id)
	if err != nil || video == nil || video.Status != want {
		t.Fatalf("video=%+v, err=%v; want status %v", video, err, want)
	}
}

func requirePublishTime(t *testing.T, store app.ScheduleStore, id uint, want *time.Time) {
	t.Helper()
	when, err := store.PublishTime(t.Context(), id)
	if err != nil || (when == nil) != (want == nil) || (when != nil && !when.Equal(*want)) {
		t.Fatalf("publish time=%v, want=%v, err=%v", when, want, err)
	}
}

func TestRevisionPublishTimeSurvivesRestart(t *testing.T) {
	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusPublished)
	future := time.Now().Add(time.Hour)
	if _, err := f.revisionService(f.videos, &revisionEffects{}).UpdateVideo(t.Context(), "alice", app.UpdateVideoOpts{VideoID: id, PublishTime: &future}); err != nil {
		t.Fatal(err)
	}
	// 重新装配模拟进程重启，计划直接从视频行读取。
	store := infra.NewScheduleStore(f.db)
	requirePublishTime(t, store, id, &future)
	requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)
	var row po.Video
	if err := f.db.First(&row, id).Error; err != nil || row.PublishAt == nil || !row.PublishAt.Equal(future) {
		t.Fatalf("publish time not saved on video: publishAt=%v, err=%v", row.PublishAt, err)
	}
	past := time.Now().Add(-time.Minute)
	if err := store.Add(t.Context(), id, past); err != nil {
		t.Fatal(err)
	}
	if ids, err := store.DueVideoIDs(t.Context(), time.Now(), []enum.VideoStatus{enum.VideoStatusScheduled}); err != nil || len(ids) != 1 || ids[0] != id {
		t.Fatalf("ids=%v, err=%v", ids, err)
	}
	if drop, err := scheduleRunner(f, store, false).ProcessDueScheduledVideo(t.Context(), id); err != nil || !drop {
		t.Fatalf("drop=%v, err=%v", drop, err)
	}
	requireScheduleStatus(t, f, id, enum.VideoStatusPublished)
	requirePublishTime(t, store, id, &past)
	var released po.Video
	if err := f.db.First(&released, id).Error; err != nil || !released.IsScheduled || released.ApprovedAt != nil {
		t.Fatalf("automatic release lost its type or fabricated approval time: video=%+v, err=%v", released, err)
	}
	if ids, err := store.DueVideoIDs(t.Context(), time.Now(), []enum.VideoStatus{enum.VideoStatusScheduled}); err != nil || len(ids) != 0 {
		t.Fatalf("published video was polled again: ids=%v, err=%v", ids, err)
	}
	if _, err := scheduleRunner(f, store, false).ProcessDueScheduledVideo(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	requirePublishTime(t, store, id, &past)
}

func TestRevisionPublishTimeWriteFailureRollsBackVideo(t *testing.T) {
	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusPublished)
	if err := f.db.Exec(`CREATE TRIGGER reject_schedule BEFORE UPDATE OF publish_at ON videos BEGIN SELECT RAISE(ABORT, 'publish time unavailable'); END`).Error; err != nil {
		t.Fatal(err)
	}
	future, title := time.Now().Add(time.Hour), "new"
	effects := &revisionEffects{}
	_, err := f.revisionService(f.videos, effects).UpdateVideo(t.Context(), "alice", app.UpdateVideoOpts{VideoID: id, Title: &title, PublishTime: &future})
	if err == nil {
		t.Fatal("publish time failure reported success")
	}
	video, err := f.videos.FindByID(t.Context(), id)
	if err != nil || video.Title != "old" || video.Status != enum.VideoStatusPublished || effects.invalidated != 0 {
		t.Fatalf("video=%+v, effects=%+v, err=%v", video, effects, err)
	}
	requirePublishTime(t, infra.NewScheduleStore(f.db), id, nil)
}

func TestScheduledPublishConsumptionFailureCanRetry(t *testing.T) {
	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusScheduled)
	store := infra.NewScheduleStore(f.db)
	past := time.Now().Add(-time.Minute)
	if err := store.Add(t.Context(), id, past); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(`CREATE TRIGGER reject_consumption BEFORE UPDATE OF status ON videos WHEN NEW.status = 1 BEGIN SELECT RAISE(ABORT, 'publish unavailable'); END`).Error; err != nil {
		t.Fatal(err)
	}
	runner := scheduleRunner(f, store, false)
	if _, err := runner.ProcessDueScheduledVideo(t.Context(), id); err == nil {
		t.Fatal("consumption failure reported success")
	}
	requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)
	requirePublishTime(t, store, id, &past)
	if err := f.db.Exec("DROP TRIGGER reject_consumption").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := runner.ProcessDueScheduledVideo(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	requireScheduleStatus(t, f, id, enum.VideoStatusPublished)
	requirePublishTime(t, store, id, &past)
}

func TestStaleDueTaskDoesNotPublishOrEraseReschedule(t *testing.T) {
	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusScheduled)
	store := infra.NewScheduleStore(f.db)
	if err := store.Add(t.Context(), id, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	ids, err := store.DueVideoIDs(t.Context(), time.Now(), []enum.VideoStatus{enum.VideoStatusScheduled})
	if err != nil || len(ids) != 1 {
		t.Fatalf("ids=%v, err=%v", ids, err)
	}
	future := time.Now().Add(time.Hour)
	if _, err := f.revisionService(f.videos, &revisionEffects{}).UpdateVideo(t.Context(), "alice", app.UpdateVideoOpts{VideoID: id, PublishTime: &future}); err != nil {
		t.Fatal(err)
	}
	if drop, err := scheduleRunner(f, store, false).ProcessDueScheduledVideo(t.Context(), id); err != nil || drop {
		t.Fatalf("stale worker consumed replacement: drop=%v, err=%v", drop, err)
	}
	requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)
	requirePublishTime(t, store, id, &future)
}

func TestMetadataRevisionPreservesTimeAndPrivateRevisionClearsIt(t *testing.T) {
	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusScheduled)
	store := infra.NewScheduleStore(f.db)
	future := time.Now().Add(time.Hour)
	if err := store.Add(t.Context(), id, future); err != nil {
		t.Fatal(err)
	}
	title := "new title"
	service := f.revisionService(f.videos, &revisionEffects{})
	if _, err := service.UpdateVideo(t.Context(), "alice", app.UpdateVideoOpts{VideoID: id, Title: &title}); err != nil {
		t.Fatal(err)
	}
	requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)
	requirePublishTime(t, store, id, &future)
	private := true
	if _, err := service.UpdateVideo(t.Context(), "alice", app.UpdateVideoOpts{VideoID: id, IsPrivate: &private}); err != nil {
		t.Fatal(err)
	}
	requireScheduleStatus(t, f, id, enum.VideoStatusPrivate)
	requirePublishTime(t, store, id, nil)
}

func TestScheduledPublishKeepsReviewAndTerminalVisibility(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status enum.VideoStatus
		review bool
		want   enum.VideoStatus
	}{
		{"review enabled", enum.VideoStatusReview, true, enum.VideoStatusReview},
		{"review disabled", enum.VideoStatusReview, false, enum.VideoStatusPublished},
		{"private", enum.VideoStatusPrivate, false, enum.VideoStatusPrivate},
		{"deleted", enum.VideoStatusDeleted, false, enum.VideoStatusDeleted},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newRevisionFixture(t)
			id := scheduleVideo(t, f, tt.status)
			store := infra.NewScheduleStore(f.db)
			past := time.Now().Add(-time.Minute)
			if err := store.Add(t.Context(), id, past); err != nil {
				t.Fatal(err)
			}
			wantDrop := !(tt.status == enum.VideoStatusReview && tt.review)
			if drop, err := scheduleRunner(f, store, tt.review).ProcessDueScheduledVideo(t.Context(), id); err != nil || drop != wantDrop {
				t.Fatalf("drop=%v, err=%v", drop, err)
			}
			requireScheduleStatus(t, f, id, tt.want)
			if tt.status == enum.VideoStatusReview {
				requirePublishTime(t, store, id, &past)
			} else {
				requirePublishTime(t, store, id, nil)
			}
		})
	}
}

func TestPublishPersistsTimeWithVideo(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint("fail=", fail), func(t *testing.T) {
			f := newRevisionFixture(t)
			if fail {
				if err := f.db.Exec(`CREATE TRIGGER reject_publish_time BEFORE INSERT ON videos WHEN NEW.publish_at IS NOT NULL BEGIN SELECT RAISE(ABORT, 'publish time unavailable'); END`).Error; err != nil {
					t.Fatal(err)
				}
			}
			future := time.Now().Add(time.Hour)
			draft := &domain.PublishDraft{Video: &domain.Video{Title: "scheduled", AuthorUsername: "alice", Status: enum.VideoStatusScheduled}, PublishAt: &future}
			err := f.videos.Persist(t.Context(), draft)
			if (err != nil) != fail {
				t.Fatalf("error=%v, want fail=%v", err, fail)
			}
			var videos int64
			if err := f.db.Model(&po.Video{}).Count(&videos).Error; err != nil {
				t.Fatal(err)
			}
			if fail {
				if videos != 0 {
					t.Fatalf("failed publish left %d videos", videos)
				}
			} else {
				if videos != 1 {
					t.Fatalf("publish created %d videos", videos)
				}
				requirePublishTime(t, infra.NewScheduleStore(f.db), draft.Video.ID, &future)
			}
		})
	}
}

type inactiveRevisionMeta struct{ app.DetailMetaCache }

func (inactiveRevisionMeta) Available() bool { return false }

type failingPublishTimeStore struct{ app.ScheduleStore }

func (failingPublishTimeStore) PublishTime(context.Context, uint) (*time.Time, error) {
	return nil, errors.New("publish time read failed")
}

func TestReviewUsesVideoPublishTime(t *testing.T) {
	future, past := time.Now().Add(time.Hour), time.Now().Add(-time.Hour)
	for _, tt := range []struct {
		name string
		when *time.Time
		fail bool
		want enum.VideoStatus
	}{
		{"future", &future, false, enum.VideoStatusScheduled},
		{"past", &past, false, enum.VideoStatusPublished},
		{"immediate", nil, false, enum.VideoStatusPublished},
		{"read failure", &future, true, enum.VideoStatusReview},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newRevisionFixture(t)
			id := scheduleVideo(t, f, enum.VideoStatusReview)
			var store app.ScheduleStore = infra.NewScheduleStore(f.db)
			if tt.when != nil {
				if err := store.Add(t.Context(), id, *tt.when); err != nil {
					t.Fatal(err)
				}
			}
			if tt.fail {
				store = failingPublishTimeStore{store}
			}
			authors := app.NewAuthorService(f.videos, revisionAuthorIDs{}, revisionMeta{effects: &revisionEffects{}}, nil, zap.NewNop())
			admin := app.NewAdminVideoService(gormx.NewTxManager(f.db), f.videos, nil, nil, authors, inactiveRevisionMeta{}, nil, store, app.NewVectorSyncService(nil, nil, nil), nil, nil, nil, nil, zap.NewNop())
			_, err := admin.Review(t.Context(), app.AdminVideoReviewParams{VideoIDs: []uint{id}, Status: enum.VideoStatusPublished})
			if (err != nil) != tt.fail {
				t.Fatalf("error=%v, want fail=%v", err, tt.fail)
			}
			requireScheduleStatus(t, f, id, tt.want)
			var stored po.Video
			if err := f.db.First(&stored, id).Error; err != nil {
				t.Fatal(err)
			}
			if (stored.ApprovedAt == nil) != tt.fail {
				t.Fatalf("approval timestamp=%v, failed=%v", stored.ApprovedAt, tt.fail)
			}
			if !tt.fail {
				requirePublishTime(t, store, id, tt.when)
			}
		})
	}
}

func TestImmediatePublishHasNoScheduledTime(t *testing.T) {
	f := newRevisionFixture(t)
	draft := &domain.PublishDraft{Video: &domain.Video{Title: "immediate", AuthorUsername: "alice", Status: enum.VideoStatusReview}}
	if err := f.videos.Persist(t.Context(), draft); err != nil {
		t.Fatal(err)
	}
	requirePublishTime(t, infra.NewScheduleStore(f.db), draft.Video.ID, nil)
}

type deletingVideoRepository struct{ domain.VideoRepository }

func (deletingVideoRepository) UsernameByUserID(context.Context, uint) (string, error) {
	return "alice", nil
}

type deletionComments struct{}

func (deletionComments) PurgeByVideoIDs(context.Context, []uint) (int64, int64, error) {
	return 0, 0, nil
}

func TestDeleteScheduleFailureRollsBackVisibility(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprint("admin=", admin), func(t *testing.T) {
			f := newRevisionFixture(t)
			id := scheduleVideo(t, f, enum.VideoStatusScheduled)
			store := infra.NewScheduleStore(f.db)
			future := time.Now().Add(time.Hour)
			if err := store.Add(t.Context(), id, future); err != nil {
				t.Fatal(err)
			}
			if err := f.db.Exec(`CREATE TRIGGER reject_cancellation BEFORE UPDATE OF publish_at ON videos WHEN NEW.publish_at IS NULL BEGIN SELECT RAISE(ABORT, 'cancel unavailable'); END`).Error; err != nil {
				t.Fatal(err)
			}
			var err error
			if admin {
				svc := app.NewAdminVideoService(gormx.NewTxManager(f.db), f.videos, nil, nil, nil, nil, nil, store, nil, nil, nil, nil, nil, zap.NewNop())
				_, err = svc.Delete(t.Context(), []uint{id}, false)
			} else {
				svc := app.NewDeleteService(gormx.NewTxManager(f.db), deletingVideoRepository{f.videos}, deletionComments{}, nil, store, nil, zap.NewNop())
				err = svc.DeleteVideo(t.Context(), id, 1)
			}
			if err == nil {
				t.Fatal("failed cancellation was ignored")
			}
			requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)
			requirePublishTime(t, store, id, &future)
		})
	}
}

func TestPublishTimeMigrationPreservesExistingVideos(t *testing.T) {
	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusPublished)
	if err := f.db.Migrator().DropIndex(&po.Video{}, "idx_videos_publish_at"); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"PublishAt", "ApprovedAt", "IsScheduled"} {
		if err := f.db.Migrator().DropColumn(&po.Video{}, column); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.db.AutoMigrate(&po.Video{}); err != nil {
		t.Fatal(err)
	}
	if !f.db.Migrator().HasColumn(&po.Video{}, "PublishAt") || !f.db.Migrator().HasColumn(&po.Video{}, "ApprovedAt") || !f.db.Migrator().HasColumn(&po.Video{}, "IsScheduled") || !f.db.Migrator().HasIndex(&po.Video{}, "idx_videos_publish_at") {
		t.Fatal("publish time column or due-query index missing")
	}
	if f.db.Migrator().HasTable("video_publish_schedules") {
		t.Fatal("unexpected separate schedule table")
	}
	requireScheduleStatus(t, f, id, enum.VideoStatusPublished)
	requirePublishTime(t, infra.NewScheduleStore(f.db), id, nil)
}
