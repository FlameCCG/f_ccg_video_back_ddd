package dynamic_test

import (
	"context"
	"errors"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/dynamic"
	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestReadProgressCannotMoveBackwards(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	base := time.Now().Add(-time.Hour).Truncate(time.Second)
	if _, err := f.reads.Create(ctx, domain.NewReadRecord(2, 1, enum.WorkTypeDynamic, 1, base)); err != nil {
		t.Fatal(err)
	}
	older, err := f.reads.FindByUserAndAuthor(ctx, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	newer, err := f.reads.FindByUserAndAuthor(ctx, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	olderAt, newerAt := base.Add(time.Minute), base.Add(2*time.Minute)
	if !older.Advance(enum.WorkTypeDynamic, 2, olderAt) || !newer.Advance(enum.WorkTypeVideo, 3, newerAt) {
		t.Fatal("invalid test setup")
	}
	for _, record := range []*domain.ReadRecord{newer, older} {
		if _, err := f.reads.Advance(ctx, record.ID(), record.LastReadWorkType(), record.LastReadWorkID(), record.LastReadWorkAt()); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.reads.FindByUserAndAuthor(ctx, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastReadWorkType() != enum.WorkTypeVideo || got.LastReadWorkID() != 3 || !got.LastReadWorkAt().Equal(newerAt) {
		t.Fatalf("read progress moved backwards: %+v; want video 3 at %v", got.Snapshot(), newerAt)
	}
}

type interleavedReadRepository struct {
	domain.ReadRepository
	afterRead func(context.Context, *domain.ReadRecord) error
}

func (r interleavedReadRepository) FindByUserAndAuthor(ctx context.Context, userID, authorID uint) (*domain.ReadRecord, error) {
	record, err := r.ReadRepository.FindByUserAndAuthor(ctx, userID, authorID)
	if err == nil {
		err = r.afterRead(ctx, record)
	}
	return record, err
}

type readCountCache struct {
	app.UnreadFollowCountCache
	invalidations int
}

func (c *readCountCache) Invalidate(uint) { c.invalidations++ }

func TestMarkLatestReadDoesNotOverwriteNewerConcurrentProgress(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	latest, err := f.activity.LatestDynamicWork(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.reads.Create(ctx, domain.NewReadRecord(2, 1, enum.WorkTypeDynamic, 1, latest.CreatedAt.Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	newerAt := latest.CreatedAt.Add(time.Hour)
	reads := interleavedReadRepository{ReadRepository: f.reads, afterRead: func(ctx context.Context, record *domain.ReadRecord) error {
		_, err := f.reads.Advance(ctx, record.ID(), enum.WorkTypeVideo, 99, newerAt)
		return err
	}}
	cache := &readCountCache{}
	unread := app.NewFollowService(f.follows, f.activity, cache, nil, zap.NewNop())
	service := app.NewReadService(reads, f.follows, f.activity, f.users, nil, unread)
	if err := service.MarkLatestRead(ctx, 2, 1); err != nil {
		t.Fatal(err)
	}
	got, err := f.reads.FindByUserAndAuthor(ctx, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastReadWorkID() != 99 || got.LastReadWorkType() != enum.WorkTypeVideo || !got.LastReadWorkAt().Equal(newerAt) {
		t.Fatalf("newer progress overwritten: %+v", got.Snapshot())
	}
	if cache.invalidations != 0 {
		t.Fatalf("stale request invalidated cache %d times", cache.invalidations)
	}
}

func TestReadProgressEqualTimeDoesNotReplaceAssociatedWork(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	saved, err := f.reads.Create(ctx, domain.NewReadRecord(2, 1, enum.WorkTypeVideo, 7, at))
	if err != nil {
		t.Fatal(err)
	}
	var before po.DynamicRead
	if err := f.db.First(&before, saved.ID()).Error; err != nil {
		t.Fatal(err)
	}
	for _, workAt := range []time.Time{at, at.Add(-time.Hour)} {
		if changed, err := f.reads.Advance(ctx, saved.ID(), enum.WorkTypeDynamic, 99, workAt); err != nil || changed {
			t.Fatalf("old/equal progress updated: changed=%v err=%v", changed, err)
		}
	}
	var after po.DynamicRead
	if err := f.db.First(&after, saved.ID()).Error; err != nil {
		t.Fatal(err)
	}
	if after.LastReadWorkID != before.LastReadWorkID || after.LastReadWorkType != before.LastReadWorkType || !after.LastReadWorkAt.Equal(before.LastReadWorkAt) || !after.UpdatedAt.Equal(before.UpdatedAt) || after.UserID != before.UserID || after.AuthorID != before.AuthorID {
		t.Fatalf("no-op changed persisted fields: before=%+v after=%+v", before, after)
	}
	if changed, err := f.reads.Advance(ctx, 9999, enum.WorkTypeVideo, 8, at.Add(time.Hour)); err != nil || changed {
		t.Fatalf("missing row: changed=%v err=%v", changed, err)
	}
}

func TestMarkLatestReadInvalidatesOnlyAfterSuccessfulWrite(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	cache := &readCountCache{}
	unread := app.NewFollowService(f.follows, f.activity, cache, nil, zap.NewNop())
	service := app.NewReadService(f.reads, f.follows, f.activity, f.users, nil, unread)
	for i := 0; i < 2; i++ {
		if err := service.MarkLatestRead(ctx, 2, 1); err != nil {
			t.Fatal(err)
		}
	}
	if cache.invalidations != 1 {
		t.Fatalf("create/replay invalidations=%d", cache.invalidations)
	}
	newerAt := time.Now().Add(time.Hour).Truncate(time.Second)
	if err := f.db.Model(&po.Dynamic{}).Where("id = 2").Update("created_at", newerAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.MarkLatestRead(ctx, 2, 1); err != nil {
		t.Fatal(err)
	}
	if cache.invalidations != 2 {
		t.Fatalf("advance invalidations=%d", cache.invalidations)
	}
	failure := errors.New("read write failed")
	if err := f.db.Callback().Update().Before("gorm:update").Register("test:reject_read_progress", func(db *gorm.DB) {
		if db.Statement.Table == "dynamic_reads" {
			db.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&po.Dynamic{}).Where("id = 2").Update("created_at", newerAt.Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.MarkLatestRead(ctx, 2, 1); !errors.Is(err, failure) {
		t.Fatalf("write error lost: %v", err)
	}
	if cache.invalidations != 2 {
		t.Fatalf("failed write invalidated cache: %d", cache.invalidations)
	}
	got, err := f.reads.FindByUserAndAuthor(ctx, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastReadWorkAt().Equal(newerAt) {
		t.Fatalf("failed write changed progress: %+v", got.Snapshot())
	}
}
