package video_test

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/video"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"

	"go.uber.org/zap"
)

func scheduleVideo(t *testing.T, f *fixture, status enum.VideoStatus) uint {
	t.Helper()

	row := po.Video{Title: testOriginalText, AuthorUsername: testAuthorAlice, Status: status}
	{
		err := f.session().Create(&row).Error
		require.NoError(t, err)
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
	require.NoError(t, err, "video=%+v, err=%v; want status %v", video, err, want)
	require.NotNil(t, video, "video=%+v, err=%v; want status %v", video, err, want)
	require.Equal(t, want, video.Status, "video=%+v, err=%v; want status %v", video, err, want)
}

func requirePublishTime(t *testing.T, store app.ScheduleStore, id uint, want *time.Time) {
	t.Helper()

	when, err := store.PublishTime(t.Context(), id)
	require.NoError(t, err, "publish time=%v, want=%v, err=%v", when, want, err)
	require.Equal(t, (when == nil), (want == nil), "publish time=%v, want=%v, err=%v", when, want, err)
	require.False(t, when != nil && !when.Equal(*want), "publish time=%v, want=%v, err=%v", when, want, err)
}

func TestRevisionPublishTimeSurvivesRestart(t *testing.T) {
	t.Parallel()

	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusPublished)

	future := time.Now().Add(time.Hour)
	{
		_, err := f.revisionService(f.videos, &revisionEffects{}).UpdateVideo(t.Context(), testAuthorAlice, app.UpdateVideoOpts{VideoID: id, PublishTime: &future})
		require.NoError(t, err)
	}
	// 重新装配模拟进程重启，计划直接从视频行读取。
	store := infra.NewScheduleStore(f.db)
	requirePublishTime(t, store, id, &future)
	requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)

	var row po.Video
	{
		err := f.db.First(&row, id).Error
		require.NoError(t, err, "publish time not saved on video: publishAt=%v, err=%v", row.PublishAt, err)
		require.NotNil(t, row.PublishAt, "publish time not saved on video: publishAt=%v, err=%v", row.PublishAt, err)
		require.True(t, row.PublishAt.Equal(future), "publish time not saved on video: publishAt=%v, err=%v", row.PublishAt, err)
	}

	past := time.Now().Add(-time.Minute)
	{
		err := store.Add(t.Context(), id, past)
		require.NoError(t, err)
	}

	{
		ids, err := store.DueVideoIDs(t.Context(), time.Now(), []enum.VideoStatus{enum.VideoStatusScheduled})
		require.NoError(t, err, "ids=%v, err=%v", ids, err)
		require.Len(t, ids, 1, "ids=%v, err=%v", ids, err)
		require.Equal(t, ids[0], id, "ids=%v, err=%v", ids, err)
	}

	{
		drop, err := scheduleRunner(f, store, false).ProcessDueScheduledVideo(t.Context(), id)
		require.NoError(t, err, "drop=%v, err=%v", drop, err)
		require.True(t, drop, "drop=%v, err=%v", drop, err)
	}

	requireScheduleStatus(t, f, id, enum.VideoStatusPublished)
	requirePublishTime(t, store, id, &past)

	var released po.Video
	{
		err := f.db.First(&released, id).Error
		require.NoError(t, err, "automatic release lost its type or fabricated approval time: video=%+v, err=%v", released, err)
		require.True(t, released.IsScheduled, "automatic release lost its type or fabricated approval time: video=%+v, err=%v", released, err)
		require.Nil(t, released.ApprovedAt, "automatic release lost its type or fabricated approval time: video=%+v, err=%v", released, err)
	}

	{
		ids, err := store.DueVideoIDs(t.Context(), time.Now(), []enum.VideoStatus{enum.VideoStatusScheduled})
		require.NoError(t, err, "published video was polled again: ids=%v, err=%v", ids, err)
		require.Empty(t, ids, "published video was polled again: ids=%v, err=%v", ids, err)
	}

	{
		_, err := scheduleRunner(f, store, false).ProcessDueScheduledVideo(t.Context(), id)
		require.NoError(t, err)
	}

	requirePublishTime(t, store, id, &past)
}

func TestRevisionPublishTimeWriteFailureRollsBackVideo(t *testing.T) {
	t.Parallel()

	f := newRevisionFixture(t)

	id := scheduleVideo(t, f, enum.VideoStatusPublished)
	{
		err := f.db.Exec(`CREATE TRIGGER reject_schedule BEFORE UPDATE OF publish_at ON videos BEGIN SELECT RAISE(ABORT, 'publish time unavailable'); END`).Error
		require.NoError(t, err)
	}

	future, title := time.Now().Add(time.Hour), testNew
	effects := &revisionEffects{}

	_, err := f.revisionService(f.videos, effects).UpdateVideo(t.Context(), testAuthorAlice, app.UpdateVideoOpts{VideoID: id, Title: &title, PublishTime: &future})
	require.Error(t, err, "publish time failure reported success")

	video, err := f.videos.FindByID(t.Context(), id)
	require.NoError(t, err, "video=%+v, effects=%+v, err=%v", video, effects, err)
	require.Equal(t, testOriginalText, video.Title, "video=%+v, effects=%+v, err=%v", video, effects, err)
	require.Equal(t, enum.VideoStatusPublished, video.Status, "video=%+v, effects=%+v, err=%v", video, effects, err)
	require.Equal(t, 0, effects.invalidated, "video=%+v, effects=%+v, err=%v", video, effects, err)

	requirePublishTime(t, infra.NewScheduleStore(f.db), id, nil)
}

func TestScheduledPublishConsumptionFailureCanRetry(t *testing.T) {
	t.Parallel()

	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusScheduled)
	store := infra.NewScheduleStore(f.db)

	past := time.Now().Add(-time.Minute)
	{
		err := store.Add(t.Context(), id, past)
		require.NoError(t, err)
	}

	{
		err := f.db.Exec(`CREATE TRIGGER reject_consumption BEFORE UPDATE OF status ON videos WHEN NEW.status = 1 BEGIN SELECT RAISE(ABORT, 'publish unavailable'); END`).Error
		require.NoError(t, err)
	}

	runner := scheduleRunner(f, store, false)
	{
		_, err := runner.ProcessDueScheduledVideo(t.Context(), id)
		require.Error(t, err, "consumption failure reported success")
	}

	requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)
	requirePublishTime(t, store, id, &past)

	{
		err := f.db.Exec("DROP TRIGGER reject_consumption").Error
		require.NoError(t, err)
	}

	{
		_, err := runner.ProcessDueScheduledVideo(t.Context(), id)
		require.NoError(t, err)
	}

	requireScheduleStatus(t, f, id, enum.VideoStatusPublished)
	requirePublishTime(t, store, id, &past)
}

func TestStaleDueTaskDoesNotPublishOrEraseReschedule(t *testing.T) {
	t.Parallel()

	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusScheduled)

	store := infra.NewScheduleStore(f.db)
	{
		err := store.Add(t.Context(), id, time.Now().Add(-time.Minute))
		require.NoError(t, err)
	}

	ids, err := store.DueVideoIDs(t.Context(), time.Now(), []enum.VideoStatus{enum.VideoStatusScheduled})
	require.NoError(t, err, "ids=%v, err=%v", ids, err)
	require.Len(t, ids, 1, "ids=%v, err=%v", ids, err)

	future := time.Now().Add(time.Hour)
	{
		_, err := f.revisionService(f.videos, &revisionEffects{}).UpdateVideo(t.Context(), testAuthorAlice, app.UpdateVideoOpts{VideoID: id, PublishTime: &future})
		require.NoError(t, err)
	}

	{
		drop, err := scheduleRunner(f, store, false).ProcessDueScheduledVideo(t.Context(), id)
		require.NoError(t, err, "stale worker consumed replacement: drop=%v, err=%v", drop, err)
		require.False(t, drop, "stale worker consumed replacement: drop=%v, err=%v", drop, err)
	}

	requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)
	requirePublishTime(t, store, id, &future)
}

func TestMetadataRevisionPreservesTimeAndPrivateRevisionClearsIt(t *testing.T) {
	t.Parallel()

	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusScheduled)
	store := infra.NewScheduleStore(f.db)

	future := time.Now().Add(time.Hour)
	{
		err := store.Add(t.Context(), id, future)
		require.NoError(t, err)
	}

	title := "new title"

	service := f.revisionService(f.videos, &revisionEffects{})
	{
		_, err := service.UpdateVideo(t.Context(), testAuthorAlice, app.UpdateVideoOpts{VideoID: id, Title: &title})
		require.NoError(t, err)
	}

	requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)
	requirePublishTime(t, store, id, &future)

	private := true
	{
		_, err := service.UpdateVideo(t.Context(), testAuthorAlice, app.UpdateVideoOpts{VideoID: id, IsPrivate: &private})
		require.NoError(t, err)
	}

	requireScheduleStatus(t, f, id, enum.VideoStatusPrivate)
	requirePublishTime(t, store, id, nil)
}

func TestScheduledPublishKeepsReviewAndTerminalVisibility(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			f := newRevisionFixture(t)
			id := scheduleVideo(t, f, tt.status)
			store := infra.NewScheduleStore(f.db)

			past := time.Now().Add(-time.Minute)
			{
				err := store.Add(t.Context(), id, past)
				require.NoError(t, err)
			}

			wantDrop := tt.status != enum.VideoStatusReview || !tt.review
			{
				drop, err := scheduleRunner(f, store, tt.review).ProcessDueScheduledVideo(t.Context(), id)
				require.NoError(t, err, "drop=%v, err=%v", drop, err)
				require.Equal(t, wantDrop, drop, "drop=%v, err=%v", drop, err)
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
	t.Parallel()

	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint("fail=", fail), func(t *testing.T) {
			f := newRevisionFixture(t)

			if fail {
				{
					err := f.db.Exec(`CREATE TRIGGER reject_publish_time BEFORE INSERT ON videos WHEN NEW.publish_at IS NOT NULL BEGIN SELECT RAISE(ABORT, 'publish time unavailable'); END`).Error
					require.NoError(t, err)
				}
			}

			future := time.Now().Add(time.Hour)
			draft := &domain.PublishDraft{Video: &domain.Video{Title: "scheduled", AuthorUsername: testAuthorAlice, Status: enum.VideoStatusScheduled}, PublishAt: &future}

			err := f.videos.Persist(t.Context(), draft)
			require.Equal(t, (err != nil), fail, "error=%v, want fail=%v", err, fail)

			var videos int64
			{
				err := f.db.Model(&po.Video{}).Count(&videos).Error
				require.NoError(t, err)
			}

			if fail {
				require.EqualValues(t, 0, videos, "failed publish left %d videos", videos)
			} else {
				require.EqualValues(t, 1, videos, "publish created %d videos", videos)

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
	t.Parallel()

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
			t.Parallel()

			f := newRevisionFixture(t)
			id := scheduleVideo(t, f, enum.VideoStatusReview)

			var store app.ScheduleStore = infra.NewScheduleStore(f.db)

			if tt.when != nil {
				{
					err := store.Add(t.Context(), id, *tt.when)
					require.NoError(t, err)
				}
			}

			if tt.fail {
				store = failingPublishTimeStore{store}
			}

			authors := app.NewAuthorService(f.videos, revisionAuthorIDs{}, revisionMeta{effects: &revisionEffects{}}, nil, zap.NewNop())
			admin := app.NewAdminVideoService(gormx.NewTxManager(f.db), f.videos, nil, nil, authors, inactiveRevisionMeta{}, nil, store, app.NewVectorSyncService(nil, nil, nil), nil, nil, nil, nil, zap.NewNop())

			_, err := admin.Review(t.Context(), app.AdminVideoReviewParams{VideoIDs: []uint{id}, Status: enum.VideoStatusPublished})
			require.Equal(t, (err != nil), tt.fail, "error=%v, want fail=%v", err, tt.fail)

			requireScheduleStatus(t, f, id, tt.want)

			var stored po.Video
			{
				err := f.db.First(&stored, id).Error
				require.NoError(t, err)
			}

			require.Equal(t, (stored.ApprovedAt == nil), tt.fail, "approval timestamp=%v, failed=%v", stored.ApprovedAt, tt.fail)

			if !tt.fail {
				requirePublishTime(t, store, id, tt.when)
			}
		})
	}
}

func TestImmediatePublishHasNoScheduledTime(t *testing.T) {
	t.Parallel()

	f := newRevisionFixture(t)

	draft := &domain.PublishDraft{Video: &domain.Video{Title: "immediate", AuthorUsername: testAuthorAlice, Status: enum.VideoStatusReview}}
	{
		err := f.videos.Persist(t.Context(), draft)
		require.NoError(t, err)
	}

	requirePublishTime(t, infra.NewScheduleStore(f.db), draft.Video.ID, nil)
}

type deletingVideoRepository struct{ domain.Repository }

func (deletingVideoRepository) UsernameByUserID(context.Context, uint) (string, error) {
	return testAuthorAlice, nil
}

type deletionComments struct{}

func (deletionComments) PurgeByVideoIDs(context.Context, []uint) (int64, int64, error) {
	return 0, 0, nil
}

func TestDeleteScheduleFailureRollsBackVisibility(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprint("admin=", admin), func(t *testing.T) {
			f := newRevisionFixture(t)
			id := scheduleVideo(t, f, enum.VideoStatusScheduled)
			store := infra.NewScheduleStore(f.db)

			future := time.Now().Add(time.Hour)
			{
				err := store.Add(t.Context(), id, future)
				require.NoError(t, err)
			}

			{
				err := f.db.Exec(`CREATE TRIGGER reject_cancellation BEFORE UPDATE OF publish_at ON videos WHEN NEW.publish_at IS NULL BEGIN SELECT RAISE(ABORT, 'cancel unavailable'); END`).Error
				require.NoError(t, err)
			}

			var err error

			if admin {
				svc := app.NewAdminVideoService(gormx.NewTxManager(f.db), f.videos, nil, nil, nil, nil, nil, store, nil, nil, nil, nil, nil, zap.NewNop())
				_, err = svc.Delete(t.Context(), []uint{id}, false)
			} else {
				svc := app.NewDeleteService(gormx.NewTxManager(f.db), deletingVideoRepository{f.videos}, deletionComments{}, nil, store, nil, zap.NewNop())
				err = svc.DeleteVideo(t.Context(), id, 1)
			}

			require.Error(t, err, "failed cancellation was ignored")

			requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)
			requirePublishTime(t, store, id, &future)
		})
	}
}

func TestPublishTimeMigrationPreservesExistingVideos(t *testing.T) {
	t.Parallel()

	f := newRevisionFixture(t)

	id := scheduleVideo(t, f, enum.VideoStatusPublished)
	{
		err := f.db.Migrator().DropIndex(&po.Video{}, "idx_videos_publish_at")
		require.NoError(t, err)
	}

	for _, column := range []string{"PublishAt", "ApprovedAt", "IsScheduled"} {
		{
			err := f.db.Migrator().DropColumn(&po.Video{}, column)
			require.NoError(t, err)
		}
	}

	{
		err := f.db.AutoMigrate(&po.Video{})
		require.NoError(t, err)
	}

	require.True(t, f.db.Migrator().HasColumn(&po.Video{}, "PublishAt"), "publish time column or due-query index missing")
	require.True(t, f.db.Migrator().HasColumn(&po.Video{}, "ApprovedAt"), "publish time column or due-query index missing")
	require.True(t, f.db.Migrator().HasColumn(&po.Video{}, "IsScheduled"), "publish time column or due-query index missing")
	require.True(t, f.db.Migrator().HasIndex(&po.Video{}, "idx_videos_publish_at"), "publish time column or due-query index missing")

	require.False(t, f.db.Migrator().HasTable("video_publish_schedules"), "unexpected separate schedule table")

	requireScheduleStatus(t, f, id, enum.VideoStatusPublished)
	requirePublishTime(t, infra.NewScheduleStore(f.db), id, nil)
}
