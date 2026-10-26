package video_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newRevisionFixture(t *testing.T) *fixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "revision.db")), &gorm.Config{
		IgnoreRelationshipsWhenMigrating:         true,
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	// 只建立改稿所需表，避免其它业务表在 SQLite 中的同名索引冲突。
	if err := db.AutoMigrate(&po.Video{}, &po.Tag{}, &po.VideoPartition{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE video_tags (video_id integer, tag_id integer, PRIMARY KEY (video_id, tag_id))").Error; err != nil {
		t.Fatal(err)
	}
	return &fixture{db: db, videos: infra.NewVideoRepository(db), partitions: infra.NewPartitionRepository(db)}
}

type revisionEffects struct {
	app.ScheduleStore
	added, removed, invalidated int
}

func (s *revisionEffects) Add(ctx context.Context, id uint, when time.Time) error {
	s.added++
	return s.ScheduleStore.Add(ctx, id, when)
}
func (s *revisionEffects) Remove(ctx context.Context, id uint) error {
	s.removed++
	return s.ScheduleStore.Remove(ctx, id)
}

type revisionMeta struct {
	app.DetailMetaCache
	effects *revisionEffects
}

func (c revisionMeta) Invalidate(context.Context, uint) { c.effects.invalidated++ }

type revisionAuthorIDs struct{ app.AuthorIDCache }

func (revisionAuthorIDs) Invalidate(context.Context, uint) {}

// 除失败注入外，使用真实仓储；锁定读取和保存必须拿到同一个事务句柄。
type revisionRepository struct {
	domain.Repository
	readCtx  context.Context
	failTags error
}

func (r *revisionRepository) FindByIDAndAuthorForUpdate(ctx context.Context, id uint, author string) (*domain.Video, error) {
	if gormx.TxFrom(ctx) == nil {
		return nil, errors.New("revision read outside transaction")
	}
	r.readCtx = ctx
	return r.Repository.FindByIDAndAuthorForUpdate(ctx, id, author)
}
func (r *revisionRepository) SaveRevision(ctx context.Context, v *domain.Video) error {
	if r.readCtx != ctx || gormx.TxFrom(ctx) == nil {
		return errors.New("revision write outside read transaction")
	}
	return r.Repository.SaveRevision(ctx, v)
}
func (r *revisionRepository) ReplaceTags(ctx context.Context, id uint, tags []domain.Tag) error {
	if r.failTags != nil {
		return r.failTags
	}
	return r.Repository.ReplaceTags(ctx, id, tags)
}

func (f *fixture) revisionService(repo domain.Repository, effects *revisionEffects) *app.UpdateService {
	effects.ScheduleStore = infra.NewScheduleStore(f.db)
	authors := app.NewAuthorService(repo, revisionAuthorIDs{}, revisionMeta{effects: effects}, nil, zap.NewNop())
	return app.NewUpdateService(gormx.NewTxManager(f.db), repo, f.partitions, effects, authors,
		app.NewVectorSyncService(nil, nil, nil), infraconfig.NewProvider(&config.Config{}))
}

func TestUpdateRevisionPersistsZeroValuesAndClearsTags(t *testing.T) {
	f := newRevisionFixture(t)
	row := po.Video{Title: "old", Description: "old description", AuthorUsername: "alice", Status: enum.VideoStatusPublished, IsOriginal: true, Views: 91, Likes: 7}
	if err := f.session().Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	tag, err := f.videos.CreateOrGetTag(t.Context(), "tag")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.videos.ReplaceTags(t.Context(), row.ID, []domain.Tag{*tag}); err != nil {
		t.Fatal(err)
	}
	effects := &revisionEffects{}
	repo := &revisionRepository{Repository: f.videos}
	blank, original := "", false
	out, err := f.revisionService(repo, effects).UpdateVideo(t.Context(), "alice", app.UpdateVideoOpts{
		VideoID: row.ID, Description: &blank, IsOriginal: &original, TagIDs: []uint{},
	})
	if err != nil {
		t.Fatal(err)
	}
	var stored po.Video
	if err := f.db.First(&stored, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Description != "" || stored.IsOriginal || stored.Views != 91 || stored.Likes != 7 || stored.Title != "old" {
		t.Fatalf("incorrect saved revision: %+v", stored)
	}
	tags, err := f.videos.TagsByVideoID(t.Context(), row.ID)
	if err != nil || len(tags) != 0 || len(out.Tags) != 0 {
		t.Fatalf("tags=%v, response=%v, err=%v", tags, out.Tags, err)
	}
	if effects.invalidated != 1 || effects.added != 0 || effects.removed != 0 {
		t.Fatalf("effects=%+v", effects)
	}
}

func TestUpdateRevisionRollsBackBeforeSideEffects(t *testing.T) {
	f := newRevisionFixture(t)
	row := po.Video{Title: "old", AuthorUsername: "alice", Status: enum.VideoStatusPublished}
	if err := f.session().Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	failure := errors.New("tag save failed")
	repo := &revisionRepository{Repository: f.videos, failTags: failure}
	effects := &revisionEffects{}
	title, private := "new", true
	_, err := f.revisionService(repo, effects).UpdateVideo(t.Context(), "alice", app.UpdateVideoOpts{
		VideoID: row.ID, Title: &title, IsPrivate: &private, TagIDs: []uint{},
	})
	if !errors.Is(err, failure) {
		t.Fatalf("error=%v", err)
	}
	stored, err := f.videos.FindByID(t.Context(), row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Title != "old" || stored.Status != enum.VideoStatusPublished {
		t.Fatalf("rollback failed: %+v", stored)
	}
	var persistedRow po.Video
	if err := f.db.First(&persistedRow, stored.ID).Error; err != nil {
		t.Fatal(err)
	}
	if effects.invalidated != 0 || persistedRow.PublishAt != nil {
		t.Fatalf("cache invalidated or publish time escaped rollback: effects=%+v, publishAt=%v", effects, persistedRow.PublishAt)
	}
}

func TestUpdateRevisionVisibilityAndOwnership(t *testing.T) {
	for _, tt := range []struct {
		name           string
		status         enum.VideoStatus
		author         string
		want           enum.VideoStatus
		wantErr        bool
		added, removed int
	}{
		{"reschedule published", enum.VideoStatusPublished, "alice", enum.VideoStatusScheduled, false, 1, 0},
		{"keep private", enum.VideoStatusPrivate, "alice", enum.VideoStatusPrivate, false, 0, 1},
		{"deleted", enum.VideoStatusDeleted, "alice", enum.VideoStatusDeleted, true, 0, 0},
		{"other author", enum.VideoStatusPublished, "bob", enum.VideoStatusPublished, true, 0, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newRevisionFixture(t)
			row := po.Video{Title: "old", AuthorUsername: "alice", Status: tt.status}
			if err := f.session().Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			effects := &revisionEffects{}
			future := time.Now().Add(time.Hour)
			_, err := f.revisionService(&revisionRepository{Repository: f.videos}, effects).UpdateVideo(t.Context(), tt.author,
				app.UpdateVideoOpts{VideoID: row.ID, PublishTime: &future})
			if tt.wantErr && !errors.Is(err, domain.ErrRecordNotFound) {
				t.Fatalf("error=%v", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatal(err)
			}
			stored, err := f.videos.FindByID(t.Context(), row.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Status != tt.want || effects.added != tt.added || effects.removed != tt.removed {
				t.Fatalf("status=%v, effects=%+v", stored.Status, effects)
			}
		})
	}
}

func TestSaveRevisionPreservesIndependentStatistics(t *testing.T) {
	f := newRevisionFixture(t)
	row := po.Video{Title: "old", AuthorUsername: "alice", Views: 80, Likes: 8, Duration: 60, Status: enum.VideoStatusPublished}
	if err := f.session().Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := gormx.NewTxManager(f.db).Do(t.Context(), func(ctx context.Context) error {
		v, err := f.videos.FindByIDAndAuthorForUpdate(ctx, row.ID, "alice")
		if err != nil {
			return err
		}
		v.Title, v.Views, v.Likes, v.Duration = "new", 0, 0, 0
		return f.videos.SaveRevision(ctx, v)
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := f.videos.FindByID(t.Context(), row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Title != "new" || stored.Views != 80 || stored.Likes != 8 || stored.Duration != 60 {
		t.Fatalf("unrelated fields overwritten: %+v", stored)
	}
}

func TestRevisionReadUsesMySQLRowLock(t *testing.T) {
	var trace bytes.Buffer
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN: "unused:unused@tcp(127.0.0.1:1)/dryrun", SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun: true, DisableAutomaticPing: true,
		Logger: logger.New(log.New(&trace, "", 0), logger.Config{LogLevel: logger.Info}),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	_, err = infra.NewVideoRepository(db).FindByIDAndAuthorForUpdate(t.Context(), 7, "alice")
	if err != nil {
		t.Fatal(err)
	}
	sql := trace.String()
	if !strings.Contains(sql, "FOR UPDATE") || !strings.Contains(sql, "author_username = 'alice'") {
		t.Fatalf("missing owner predicate or row lock: %s", sql)
	}
}
