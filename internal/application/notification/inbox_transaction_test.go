package notification_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/notification"
	notificationdomain "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/notification"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newInboxTransactionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "inbox.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.AutoMigrate(&po.UserGlobalNotification{}, &po.NotificationUserRead{}); err != nil {
		t.Fatal(err)
	}
	return db
}

type failedInboxCommit struct {
	base tx.Manager
	err  error
}

func (m failedInboxCommit) Do(ctx context.Context, action func(context.Context) error) error {
	return m.base.Do(ctx, func(txCtx context.Context) error {
		if err := action(txCtx); err != nil {
			return err
		}
		return m.err
	})
}

func TestInboxDeleteRollsBackNoticesAndReadsBeforeCacheInvalidation(t *testing.T) {
	for _, stage := range []string{"read deletion", "commit"} {
		t.Run(stage, func(t *testing.T) {
			db := newInboxTransactionDB(t)
			repo := infra.NewRepository(db)
			notice, err := repo.Create(t.Context(), notificationdomain.NewNotification(notificationdomain.Draft{
				Type: enum.NotificationVideoLikeType, ReceiverID: 7,
			}))
			if err != nil {
				t.Fatal(err)
			}
			ids := []uint{notice.ID()}
			if err := repo.MarkRead(t.Context(), 7, ids); err != nil {
				t.Fatal(err)
			}
			cache := &countCache{available: true}
			counts := app.NewCountService(repo, cache, unreadChat{}, nil, zap.NewNop())
			var manager tx.Manager = gormx.NewTxManager(db)
			failure := errors.New("commit unavailable")
			if stage == "commit" {
				manager = failedInboxCommit{manager, failure}
			} else if err := db.Exec(`CREATE TRIGGER reject_read_delete BEFORE DELETE ON notification_user_reads BEGIN SELECT RAISE(ABORT, 'read deletion unavailable'); END`).Error; err != nil {
				t.Fatal(err)
			}
			inbox := app.NewInboxService(manager, repo, counts)
			if err := inbox.Delete(t.Context(), 7, ids); err == nil || (stage == "commit" && !errors.Is(err, failure)) {
				t.Fatalf("failure lost: %v", err)
			}
			current, err := repo.FindByID(t.Context(), notice.ID())
			if err != nil || current == nil || current.Type() != enum.NotificationVideoLikeType {
				t.Fatalf("notice was not rolled back: current=%+v err=%v", current, err)
			}
			reads, err := repo.ReadSet(t.Context(), 7, ids)
			if err != nil || len(reads) != 1 || len(cache.invalidated) != 0 {
				t.Fatalf("reads/cache changed on failure: reads=%v invalidated=%v err=%v", reads, cache.invalidated, err)
			}
		})
	}
}

type trackedInboxRepository struct {
	*infra.Repository
	cache  *countCache
	locked *gorm.DB
	t      *testing.T
}

func (r *trackedInboxRepository) FindByIDsForUpdate(ctx context.Context, ids []uint) ([]notificationdomain.Notification, error) {
	r.t.Helper()
	r.locked = gormx.TxFrom(ctx)
	if r.locked == nil {
		r.t.Fatal("delete eligibility was read outside its transaction")
	}
	return r.Repository.FindByIDsForUpdate(ctx, ids)
}

func (r *trackedInboxRepository) DeleteForReceiver(ctx context.Context, receiverID int, ids []uint) error {
	r.t.Helper()
	if gormx.TxFrom(ctx) != r.locked || r.locked == nil || len(r.cache.invalidated) != 0 {
		r.t.Fatal("delete did not share its locked read transaction, or invalidated cache before commit")
	}
	return r.Repository.DeleteForReceiver(ctx, receiverID, ids)
}

func TestInboxDeleteLocksAndCommitsBeforeInvalidatingCache(t *testing.T) {
	db := newInboxTransactionDB(t)
	repo := infra.NewRepository(db)
	cache := &countCache{available: true}
	counts := app.NewCountService(repo, cache, unreadChat{}, nil, zap.NewNop())
	inbox := app.NewInboxService(gormx.NewTxManager(db), &trackedInboxRepository{Repository: repo, cache: cache, t: t}, counts)
	notice, err := repo.Create(t.Context(), notificationdomain.NewNotification(notificationdomain.Draft{
		Type: enum.NotificationVideoLikeType, ReceiverID: 7,
	}))
	if err != nil {
		t.Fatal(err)
	}
	ids := []uint{notice.ID()}
	if err := repo.MarkRead(t.Context(), 7, ids); err != nil {
		t.Fatal(err)
	}
	if err := inbox.Delete(t.Context(), 7, ids); err != nil {
		t.Fatal(err)
	}
	current, err := repo.FindByID(t.Context(), notice.ID())
	if err != nil || current != nil {
		t.Fatalf("notice survived successful delete: current=%+v err=%v", current, err)
	}
	reads, err := repo.ReadSet(t.Context(), 7, ids)
	if err != nil || len(reads) != 0 || len(cache.invalidated) != 1 || cache.invalidated[0] != 7 {
		t.Fatalf("delete effects=%v reads=%v err=%v", cache.invalidated, reads, err)
	}
	if err := inbox.Delete(t.Context(), 7, ids); err != nil || len(cache.invalidated) != 1 {
		t.Fatalf("missing notice should be a no-op: err=%v invalidated=%v", err, cache.invalidated)
	}
}

func TestInboxDeleteChecksEntireBatchBeforeWriting(t *testing.T) {
	db := newInboxTransactionDB(t)
	repo := infra.NewRepository(db)
	cache := &countCache{available: true}
	counts := app.NewCountService(repo, cache, unreadChat{}, nil, zap.NewNop())
	inbox := app.NewInboxService(gormx.NewTxManager(db), repo, counts)
	var ids []uint
	for _, kind := range []enum.Type{enum.NotificationVideoLikeType, enum.NotificationSystemType} {
		notice, err := repo.Create(t.Context(), notificationdomain.NewNotification(notificationdomain.Draft{Type: kind, ReceiverID: 7}))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, notice.ID())
	}
	if err := inbox.Delete(t.Context(), 7, ids); !errors.Is(err, notificationdomain.ErrSystemNotificationNotDeletable) {
		t.Fatalf("error contract changed: %v", err)
	}
	var total int64
	if err := db.Model(&po.UserGlobalNotification{}).Count(&total).Error; err != nil || total != 2 || len(cache.invalidated) != 0 {
		t.Fatalf("partial batch deletion: total=%d invalidated=%v err=%v", total, cache.invalidated, err)
	}
}

func TestInboxDeleteReadUsesOrderedMySQLRowLocks(t *testing.T) {
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
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := infra.NewRepository(db).FindByIDsForUpdate(t.Context(), []uint{9, 2}); err != nil {
		t.Fatal(err)
	}
	sql := trace.String()
	if !strings.Contains(sql, "FOR UPDATE") || !strings.Contains(sql, "ORDER BY id ASC") || !strings.Contains(sql, "id IN (9,2)") {
		t.Fatalf("delete target read is missing its row lock or order: %s", sql)
	}
}

type convertingInboxRepository struct{ *infra.Repository }

func (r convertingInboxRepository) FindByIDsForUpdate(ctx context.Context, ids []uint) ([]notificationdomain.Notification, error) {
	// 管理员在当前请求锁定目标之前提交转换；随后必须读取并判断最新状态。
	current, err := r.Repository.FindByID(context.Background(), ids[0])
	if err != nil {
		return nil, err
	}
	current.ReviseAsSystem(notificationdomain.SystemContent{Title: "system", ReceiverID: current.ReceiverID()})
	if _, err := r.Repository.UpdateSystemContent(context.Background(), current); err != nil {
		return nil, err
	}
	return r.Repository.FindByIDsForUpdate(ctx, ids)
}

func TestInboxDeleteRejectsConcurrentSystemConversion(t *testing.T) {
	db := newInboxTransactionDB(t)
	repo := infra.NewRepository(db)
	notice, err := repo.Create(t.Context(), notificationdomain.NewNotification(notificationdomain.Draft{
		Type: enum.NotificationVideoLikeType, ReceiverID: 7,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkRead(t.Context(), 7, []uint{notice.ID()}); err != nil {
		t.Fatal(err)
	}
	cache := &countCache{available: true}
	counts := app.NewCountService(repo, cache, unreadChat{}, nil, zap.NewNop())
	inbox := app.NewInboxService(gormx.NewTxManager(db), convertingInboxRepository{repo}, counts)
	if err := inbox.Delete(t.Context(), 7, []uint{notice.ID()}); !errors.Is(err, notificationdomain.ErrSystemNotificationNotDeletable) {
		t.Fatalf("error = %v; must reject the latest system notification", err)
	}
	current, err := repo.FindByID(t.Context(), notice.ID())
	if err != nil || current == nil || current.Type() != enum.NotificationSystemType {
		t.Fatalf("converted notification lost: current=%+v err=%v", current, err)
	}
	readSet, err := repo.ReadSet(t.Context(), 7, []uint{notice.ID()})
	if err != nil || len(readSet) != 1 || len(cache.invalidated) != 0 {
		t.Fatalf("rejected delete changed reads/cache: reads=%v invalidated=%v err=%v", readSet, cache.invalidated, err)
	}
}
