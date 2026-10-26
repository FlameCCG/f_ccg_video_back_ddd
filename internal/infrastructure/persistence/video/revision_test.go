package video_test

import (
	"bytes"
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/video"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

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
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)

	t.Cleanup(func() { _ = sqlDB.Close() })
	// 只建立改稿所需表，避免其它业务表在 SQLite 中的同名索引冲突。
	{
		err := db.AutoMigrate(&po.Video{}, &po.Tag{}, &po.VideoPartition{})
		require.NoError(t, err)
	}

	{
		err := db.Exec("CREATE TABLE video_tags (video_id integer, tag_id integer, PRIMARY KEY (video_id, tag_id))").Error
		require.NoError(t, err)
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
	readCtx  context.Context //nolint:containedctx // Captures the context to verify propagation through the publication transaction.
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
	t.Parallel()

	f := newRevisionFixture(t)

	row := po.Video{Title: testOriginalText, Description: "old description", AuthorUsername: testAuthorAlice, Status: enum.VideoStatusPublished, IsOriginal: true, Views: 91, Likes: 7}
	{
		err := f.session().Create(&row).Error
		require.NoError(t, err)
	}

	tag, err := f.videos.CreateOrGetTag(t.Context(), "tag")
	require.NoError(t, err)

	{
		err := f.videos.ReplaceTags(t.Context(), row.ID, []domain.Tag{*tag})
		require.NoError(t, err)
	}

	effects := &revisionEffects{}
	repo := &revisionRepository{Repository: f.videos}
	blank, original := "", false

	out, err := f.revisionService(repo, effects).UpdateVideo(t.Context(), testAuthorAlice, app.UpdateVideoOpts{
		VideoID: row.ID, Description: &blank, IsOriginal: &original, TagIDs: []uint{},
	})
	require.NoError(t, err)

	var stored po.Video
	{
		err := f.db.First(&stored, row.ID).Error
		require.NoError(t, err)
	}

	require.Empty(t, stored.Description, "incorrect saved revision: %+v", stored)
	require.False(t, stored.IsOriginal, "incorrect saved revision: %+v", stored)
	require.EqualValues(t, 91, stored.Views, "incorrect saved revision: %+v", stored)
	require.EqualValues(t, 7, stored.Likes, "incorrect saved revision: %+v", stored)
	require.Equal(t, testOriginalText, stored.Title, "incorrect saved revision: %+v", stored)

	tags, err := f.videos.TagsByVideoID(t.Context(), row.ID)
	require.NoError(t, err, "tags=%v, response=%v, err=%v", tags, out.Tags, err)
	require.Empty(t, tags, "tags=%v, response=%v, err=%v", tags, out.Tags, err)
	require.Empty(t, out.Tags, "tags=%v, response=%v, err=%v", tags, out.Tags, err)

	require.Equal(t, 1, effects.invalidated, "effects=%+v", effects)
	require.Equal(t, 0, effects.added, "effects=%+v", effects)
	require.Equal(t, 0, effects.removed, "effects=%+v", effects)
}

func TestUpdateRevisionRollsBackBeforeSideEffects(t *testing.T) {
	t.Parallel()

	f := newRevisionFixture(t)

	row := po.Video{Title: testOriginalText, AuthorUsername: testAuthorAlice, Status: enum.VideoStatusPublished}
	{
		err := f.session().Create(&row).Error
		require.NoError(t, err)
	}

	failure := errors.New("tag save failed")
	repo := &revisionRepository{Repository: f.videos, failTags: failure}
	effects := &revisionEffects{}
	title, private := testNew, true

	_, err := f.revisionService(repo, effects).UpdateVideo(t.Context(), testAuthorAlice, app.UpdateVideoOpts{
		VideoID: row.ID, Title: &title, IsPrivate: &private, TagIDs: []uint{},
	})
	require.ErrorIs(t, err, failure, "error=%v", err)

	stored, err := f.videos.FindByID(t.Context(), row.ID)
	require.NoError(t, err)

	require.Equal(t, testOriginalText, stored.Title, "rollback failed: %+v", stored)
	require.Equal(t, enum.VideoStatusPublished, stored.Status, "rollback failed: %+v", stored)

	var persistedRow po.Video
	{
		err := f.db.First(&persistedRow, stored.ID).Error
		require.NoError(t, err)
	}

	require.Equal(t, 0, effects.invalidated, "cache invalidated or publish time escaped rollback: effects=%+v, publishAt=%v", effects, persistedRow.PublishAt)
	require.Nil(t, persistedRow.PublishAt, "cache invalidated or publish time escaped rollback: effects=%+v, publishAt=%v", effects, persistedRow.PublishAt)
}

func TestUpdateRevisionVisibilityAndOwnership(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name           string
		status         enum.VideoStatus
		author         string
		want           enum.VideoStatus
		wantErr        bool
		added, removed int
	}{
		{"reschedule published", enum.VideoStatusPublished, testAuthorAlice, enum.VideoStatusScheduled, false, 1, 0},
		{"keep private", enum.VideoStatusPrivate, testAuthorAlice, enum.VideoStatusPrivate, false, 0, 1},
		{"deleted", enum.VideoStatusDeleted, testAuthorAlice, enum.VideoStatusDeleted, true, 0, 0},
		{"other author", enum.VideoStatusPublished, "bob", enum.VideoStatusPublished, true, 0, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newRevisionFixture(t)

			row := po.Video{Title: testOriginalText, AuthorUsername: testAuthorAlice, Status: tt.status}
			{
				err := f.session().Create(&row).Error
				require.NoError(t, err)
			}

			effects := &revisionEffects{}
			future := time.Now().Add(time.Hour)

			_, err := f.revisionService(&revisionRepository{Repository: f.videos}, effects).UpdateVideo(t.Context(), tt.author,
				app.UpdateVideoOpts{VideoID: row.ID, PublishTime: &future})
			require.False(t, tt.wantErr && !errors.Is(err, domain.ErrRecordNotFound), "error=%v", err)

			require.False(t, !tt.wantErr && err != nil, err)

			stored, err := f.videos.FindByID(t.Context(), row.ID)
			require.NoError(t, err)

			require.Equal(t, tt.want, stored.Status, "status=%v, effects=%+v", stored.Status, effects)
			require.Equal(t, effects.added, tt.added, "status=%v, effects=%+v", stored.Status, effects)
			require.Equal(t, effects.removed, tt.removed, "status=%v, effects=%+v", stored.Status, effects)
		})
	}
}

func TestSaveRevisionPreservesIndependentStatistics(t *testing.T) {
	t.Parallel()

	f := newRevisionFixture(t)

	row := po.Video{Title: testOriginalText, AuthorUsername: testAuthorAlice, Views: 80, Likes: 8, Duration: 60, Status: enum.VideoStatusPublished}
	{
		err := f.session().Create(&row).Error
		require.NoError(t, err)
	}

	{
		err := gormx.NewTxManager(f.db).Do(t.Context(), func(ctx context.Context) error {
			v, err := f.videos.FindByIDAndAuthorForUpdate(ctx, row.ID, testAuthorAlice)
			if err != nil {
				return err
			}

			v.Title, v.Views, v.Likes, v.Duration = testNew, 0, 0, 0

			return f.videos.SaveRevision(ctx, v)
		})
		require.NoError(t, err)
	}

	stored, err := f.videos.FindByID(t.Context(), row.ID)
	require.NoError(t, err)

	require.Equal(t, testNew, stored.Title, "unrelated fields overwritten: %+v", stored)
	require.EqualValues(t, 80, stored.Views, "unrelated fields overwritten: %+v", stored)
	require.EqualValues(t, 8, stored.Likes, "unrelated fields overwritten: %+v", stored)
	require.Equal(t, 60, stored.Duration, "unrelated fields overwritten: %+v", stored)
}

func TestRevisionReadUsesMySQLRowLock(t *testing.T) {
	t.Parallel()

	var trace bytes.Buffer

	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN: "unused:unused@tcp(127.0.0.1:1)/dryrun", SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun: true, DisableAutomaticPing: true,
		Logger: logger.New(log.New(&trace, "", 0), logger.Config{LogLevel: logger.Info}),
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)

	t.Cleanup(func() { _ = sqlDB.Close() })

	_, err = infra.NewVideoRepository(db).FindByIDAndAuthorForUpdate(t.Context(), 7, testAuthorAlice)
	require.NoError(t, err)

	sql := trace.String()
	require.Contains(t, sql, "FOR UPDATE", "missing owner predicate or row lock: %s", sql)
	require.Contains(t, sql, "author_username = 'alice'", "missing owner predicate or row lock: %s", sql)
}
