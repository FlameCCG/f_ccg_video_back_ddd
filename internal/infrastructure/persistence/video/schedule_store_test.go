package video_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

type legacySchedules struct {
	entries        map[uint]time.Time
	err, removeErr error
}

func (*legacySchedules) Available() bool { return true }
func (s *legacySchedules) Score(_ context.Context, id uint) (float64, bool, error) {
	when, ok := s.entries[id]
	return float64(when.Unix()), ok, s.err
}
func (s *legacySchedules) DueMembers(_ context.Context, now int64) ([]string, error) {
	var members []string
	for id, when := range s.entries {
		if when.Unix() <= now {
			members = append(members, strconv.FormatUint(uint64(id), 10))
		}
	}
	return members, s.err
}
func (s *legacySchedules) RemoveMember(_ context.Context, member string) error {
	if s.removeErr != nil {
		return s.removeErr
	}
	id, _ := strconv.ParseUint(member, 10, 64)
	delete(s.entries, uint(id))
	return s.err
}

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

func TestRevisionScheduleSurvivesRedisOutageAndRestart(t *testing.T) {
	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusPublished)
	future := time.Now().Add(time.Hour)
	effects := &revisionEffects{}
	if _, err := f.revisionService(f.videos, effects).UpdateVideo(t.Context(), "alice", app.UpdateVideoOpts{VideoID: id, PublishTime: &future}); err != nil {
		t.Fatal(err)
	}
	// 重新装配模拟进程重启，Redis 此时完全不可用。
	store := infra.NewScheduleStore(f.db, &legacySchedules{err: errors.New("redis unavailable")})
	if score, ok, err := store.Score(t.Context(), id); err != nil || !ok || int64(score) != future.Unix() {
		t.Fatalf("score=%v, ok=%v, err=%v", score, ok, err)
	}
	requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)
	past := time.Now().Add(-time.Minute)
	if err := store.Add(t.Context(), id, past); err != nil {
		t.Fatal(err)
	}
	if members, err := store.DueMembers(t.Context(), time.Now().Unix()); err != nil || len(members) != 1 {
		t.Fatalf("members=%v, err=%v", members, err)
	}
	if drop, err := scheduleRunner(f, store, false).ProcessDueScheduledVideo(t.Context(), id); err != nil || !drop {
		t.Fatalf("drop=%v, err=%v", drop, err)
	}
	requireScheduleStatus(t, f, id, enum.VideoStatusPublished)
	if _, exists, err := store.Score(t.Context(), id); err != nil || exists {
		t.Fatalf("consumed task remains: exists=%v, err=%v", exists, err)
	}
}

func TestRevisionScheduleWriteFailureRollsBackVideo(t *testing.T) {
	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusPublished)
	if err := f.db.Exec(`CREATE TRIGGER reject_schedule BEFORE INSERT ON video_publish_schedules BEGIN SELECT RAISE(ABORT, 'schedule unavailable'); END`).Error; err != nil {
		t.Fatal(err)
	}
	future, title := time.Now().Add(time.Hour), "new"
	effects := &revisionEffects{}
	_, err := f.revisionService(f.videos, effects).UpdateVideo(t.Context(), "alice", app.UpdateVideoOpts{VideoID: id, Title: &title, PublishTime: &future})
	if err == nil {
		t.Fatal("schedule failure reported success")
	}
	video, err := f.videos.FindByID(t.Context(), id)
	if err != nil || video.Title != "old" || video.Status != enum.VideoStatusPublished || effects.invalidated != 0 {
		t.Fatalf("video=%+v, effects=%+v, err=%v", video, effects, err)
	}
}

func TestScheduledPublishConsumptionFailureCanRetry(t *testing.T) {
	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusScheduled)
	store := infra.NewScheduleStore(f.db, nil)
	if err := store.Add(t.Context(), id, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(`CREATE TRIGGER reject_consumption BEFORE UPDATE OF publish_at ON video_publish_schedules WHEN NEW.publish_at IS NULL BEGIN SELECT RAISE(ABORT, 'consume unavailable'); END`).Error; err != nil {
		t.Fatal(err)
	}
	runner := scheduleRunner(f, store, false)
	if _, err := runner.ProcessDueScheduledVideo(t.Context(), id); err == nil {
		t.Fatal("consumption failure reported success")
	}
	requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)
	if _, exists, err := store.Score(t.Context(), id); err != nil || !exists {
		t.Fatalf("retry task lost: exists=%v, err=%v", exists, err)
	}
	if err := f.db.Exec("DROP TRIGGER reject_consumption").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := runner.ProcessDueScheduledVideo(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	requireScheduleStatus(t, f, id, enum.VideoStatusPublished)
}

func TestStaleDueTaskDoesNotPublishOrEraseReschedule(t *testing.T) {
	f := newRevisionFixture(t)
	id := scheduleVideo(t, f, enum.VideoStatusScheduled)
	store := infra.NewScheduleStore(f.db, nil)
	if err := store.Add(t.Context(), id, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	members, err := store.DueMembers(t.Context(), time.Now().Unix())
	if err != nil || len(members) != 1 {
		t.Fatalf("members=%v, err=%v", members, err)
	}
	future := time.Now().Add(time.Hour)
	if _, err := f.revisionService(f.videos, &revisionEffects{}).UpdateVideo(t.Context(), "alice", app.UpdateVideoOpts{VideoID: id, PublishTime: &future}); err != nil {
		t.Fatal(err)
	}
	if drop, err := scheduleRunner(f, store, false).ProcessDueScheduledVideo(t.Context(), id); err != nil || drop {
		t.Fatalf("stale worker consumed replacement: drop=%v, err=%v", drop, err)
	}
	requireScheduleStatus(t, f, id, enum.VideoStatusScheduled)
	if score, exists, err := store.Score(t.Context(), id); err != nil || !exists || int64(score) != future.Unix() {
		t.Fatalf("reschedule lost: score=%v, exists=%v, err=%v", score, exists, err)
	}
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
			store := infra.NewScheduleStore(f.db, nil)
			if err := store.Add(t.Context(), id, time.Now().Add(-time.Minute)); err != nil {
				t.Fatal(err)
			}
			if drop, err := scheduleRunner(f, store, tt.review).ProcessDueScheduledVideo(t.Context(), id); err != nil || !drop {
				t.Fatalf("drop=%v, err=%v", drop, err)
			}
			requireScheduleStatus(t, f, id, tt.want)
			if _, exists, err := store.Score(t.Context(), id); err != nil || exists {
				t.Fatalf("stale schedule remains: exists=%v, err=%v", exists, err)
			}
		})
	}
}

func TestLegacyRecoveryDoesNotReviveCancelledOrOverwriteNewSchedule(t *testing.T) {
	f := newRevisionFixture(t)
	one := scheduleVideo(t, f, enum.VideoStatusScheduled)
	two := scheduleVideo(t, f, enum.VideoStatusScheduled)
	three := scheduleVideo(t, f, enum.VideoStatusReview)
	future, later := time.Now().Add(time.Hour), time.Now().Add(2*time.Hour)
	legacy := &legacySchedules{entries: map[uint]time.Time{one: future, two: future, three: future}, removeErr: errors.New("redis delete unavailable")}
	store := infra.NewScheduleStore(f.db, legacy)
	if err := store.Remove(t.Context(), one); err != nil {
		t.Fatal(err)
	}
	if err := store.Add(t.Context(), two, later); err != nil {
		t.Fatal(err)
	}
	if err := store.RecoverLegacy(t.Context()); err == nil {
		t.Fatal("expected failed Redis cleanup")
	}
	legacy.removeErr = nil
	if err := store.RecoverLegacy(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		id     uint
		exists bool
		when   time.Time
	}{{one, false, time.Time{}}, {two, true, later}, {three, true, future}} {
		score, exists, err := store.Score(t.Context(), tt.id)
		if err != nil || exists != tt.exists || (exists && int64(score) != tt.when.Unix()) {
			t.Fatalf("id=%d score=%v exists=%v err=%v", tt.id, score, exists, err)
		}
	}
	if len(legacy.entries) != 0 {
		t.Fatalf("legacy entries not cleaned: %v", legacy.entries)
	}
}

func TestPublishPersistsScheduleAtomically(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint("fail=", fail), func(t *testing.T) {
			f := newRevisionFixture(t)
			if fail {
				if err := f.db.Exec(`CREATE TRIGGER reject_publish_schedule BEFORE INSERT ON video_publish_schedules BEGIN SELECT RAISE(ABORT, 'schedule unavailable'); END`).Error; err != nil {
					t.Fatal(err)
				}
			}
			future := time.Now().Add(time.Hour)
			draft := &domain.PublishDraft{Video: &domain.Video{Title: "scheduled", AuthorUsername: "alice", Status: enum.VideoStatusScheduled}, PublishAt: &future}
			err := f.videos.Persist(t.Context(), draft)
			if (err != nil) != fail {
				t.Fatalf("error=%v, want fail=%v", err, fail)
			}
			var videos, plans int64
			if err := f.db.Model(&po.Video{}).Count(&videos).Error; err != nil {
				t.Fatal(err)
			}
			if err := f.db.Model(&po.VideoPublishSchedule{}).Count(&plans).Error; err != nil {
				t.Fatal(err)
			}
			want := int64(1)
			if fail {
				want = 0
			}
			if videos != want || plans != want {
				t.Fatalf("partial publish: videos=%d, plans=%d", videos, plans)
			}
		})
	}
}

type inactiveRevisionMeta struct{ app.DetailMetaCache }

func (inactiveRevisionMeta) Available() bool { return false }

func TestReviewUsesDurableScheduleAndFailsClosedOnUnknownLegacyPlan(t *testing.T) {
	for _, known := range []bool{true, false} {
		t.Run(fmt.Sprint("durable=", known), func(t *testing.T) {
			f := newRevisionFixture(t)
			id := scheduleVideo(t, f, enum.VideoStatusReview)
			store := infra.NewScheduleStore(f.db, &legacySchedules{err: errors.New("redis unavailable")})
			if known {
				if err := store.Add(t.Context(), id, time.Now().Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
			}
			authors := app.NewAuthorService(f.videos, revisionAuthorIDs{}, revisionMeta{effects: &revisionEffects{}}, nil, zap.NewNop())
			admin := app.NewAdminVideoService(gormx.NewTxManager(f.db), f.videos, nil, nil, authors, inactiveRevisionMeta{}, nil, store, app.NewVectorSyncService(nil, nil, nil), nil, nil, nil, nil, zap.NewNop())
			_, err := admin.Review(t.Context(), app.AdminVideoReviewParams{VideoIDs: []uint{id}, Status: enum.VideoStatusPublished})
			want := enum.VideoStatusReview
			if known {
				want = enum.VideoStatusScheduled
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unknown legacy plan allowed early publication")
			}
			requireScheduleStatus(t, f, id, want)
		})
	}
}

func TestNewImmediateReviewDoesNotDependOnRedis(t *testing.T) {
	f := newRevisionFixture(t)
	draft := &domain.PublishDraft{Video: &domain.Video{Title: "immediate", AuthorUsername: "alice", Status: enum.VideoStatusReview}}
	if err := f.videos.Persist(t.Context(), draft); err != nil {
		t.Fatal(err)
	}
	store := infra.NewScheduleStore(f.db, &legacySchedules{err: errors.New("redis unavailable")})
	if _, exists, err := store.Score(t.Context(), draft.Video.ID); err != nil || exists {
		t.Fatalf("new immediate publish queried legacy Redis: exists=%v, err=%v", exists, err)
	}
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
			store := infra.NewScheduleStore(f.db, nil)
			future := time.Now().Add(time.Hour)
			if err := store.Add(t.Context(), id, future); err != nil {
				t.Fatal(err)
			}
			if err := f.db.Exec(`CREATE TRIGGER reject_cancellation BEFORE UPDATE OF publish_at ON video_publish_schedules WHEN NEW.publish_at IS NULL BEGIN SELECT RAISE(ABORT, 'cancel unavailable'); END`).Error; err != nil {
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
			if score, exists, err := store.Score(t.Context(), id); err != nil || !exists || int64(score) != future.Unix() {
				t.Fatalf("partial delete: score=%v, exists=%v, err=%v", score, exists, err)
			}
		})
	}
}
