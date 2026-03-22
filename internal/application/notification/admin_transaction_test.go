package notification_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/notification"
	notificationdomain "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/notification"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type adminTransactionRepository struct {
	*infra.Repository
	t          *testing.T
	cache      *countCache
	beforeLock func()
	locked     *gorm.DB
}

func (r *adminTransactionRepository) FindByIDsForUpdate(ctx context.Context, ids []uint) ([]notificationdomain.Notification, error) {
	r.t.Helper()
	r.locked = gormx.TxFrom(ctx)
	if r.locked == nil {
		r.t.Fatal("admin write read outside its transaction")
	}
	if r.beforeLock != nil {
		before := r.beforeLock
		r.beforeLock = nil
		before()
	}
	return r.Repository.FindByIDsForUpdate(ctx, ids)
}

func (r *adminTransactionRepository) ensureSameTransaction(ctx context.Context) {
	r.t.Helper()
	if r.locked == nil || r.locked != gormx.TxFrom(ctx) || len(r.cache.invalidated) != 0 {
		r.t.Fatal("admin save must share locked read transaction and defer invalidation until commit")
	}
}

func (r *adminTransactionRepository) UpdateSystemContent(ctx context.Context, notice *notificationdomain.Notification) (int, error) {
	r.ensureSameTransaction(ctx)
	return r.Repository.UpdateSystemContent(ctx, notice)
}

func (r *adminTransactionRepository) DeleteByIDs(ctx context.Context, ids []uint) error {
	r.ensureSameTransaction(ctx)
	return r.Repository.DeleteByIDs(ctx, ids)
}

func TestAdminWritesInvalidateLatestReceiverAfterInterleavedMove(t *testing.T) {
	for _, action := range []string{"update", "delete"} {
		t.Run(action, func(t *testing.T) {
			db := newInboxTransactionDB(t)
			repo := infra.NewRepository(db)
			manager := gormx.NewTxManager(db)
			cache := &countCache{available: true}
			counts := app.NewCountService(repo, cache, unreadChat{}, nil, zap.NewNop())
			notice, err := repo.Create(t.Context(), notificationdomain.NewNotification(notificationdomain.Draft{ReceiverID: 1, Type: enum.NotificationApplyType}))
			if err != nil {
				t.Fatal(err)
			}
			tracked := &adminTransactionRepository{Repository: repo, t: t, cache: cache}
			tracked.beforeLock = func() {
				// Another request commits A -> B before this request acquires the row lock.
				other := app.NewAdminService(manager, repo, nil, counts)
				if err := other.UpdateSystemNotice(t.Context(), app.UpdateSystemNoticeCommand{ID: notice.ID(), ReceiverID: 2}); err != nil {
					t.Fatal(err)
				}
				badge, err := counts.CountSummary(t.Context(), 2)
				if err != nil || badge.System != 1 {
					t.Fatalf("could not rebuild intermediate receiver's badge: %+v %v", badge, err)
				}
				cache.invalidated = nil
			}
			admin := app.NewAdminService(manager, tracked, nil, counts)
			if action == "update" {
				err = admin.UpdateSystemNotice(t.Context(), app.UpdateSystemNoticeCommand{ID: notice.ID(), ReceiverID: 3})
			} else {
				err = admin.AdminDelete(t.Context(), []uint{notice.ID()})
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, _, cached := cache.Load(2); cached {
				t.Fatal("intermediate receiver B still has a stale badge after A -> B -> C/delete")
			}
			want := []int{2}
			if action == "update" {
				want = append(want, 3)
			}
			if tracked.locked == nil || !reflect.DeepEqual(cache.invalidated, want) {
				t.Fatalf("did not invalidate latest receivers after commit: %v, want %v", cache.invalidated, want)
			}
		})
	}
}

func TestAdminWritesRollbackBeforeInvalidatingBadges(t *testing.T) {
	for _, action := range []string{"update", "delete"} {
		for _, stage := range []string{"save", "commit"} {
			t.Run(action+"/"+stage, func(t *testing.T) {
				db := newInboxTransactionDB(t)
				repo := infra.NewRepository(db)
				notice, err := repo.Create(t.Context(), notificationdomain.NewNotification(notificationdomain.Draft{ReceiverID: 1, Type: enum.NotificationApplyType, Title: "original"}))
				if err != nil {
					t.Fatal(err)
				}
				if err := repo.MarkRead(t.Context(), 1, []uint{notice.ID()}); err != nil {
					t.Fatal(err)
				}
				cache := &countCache{available: true}
				counts := app.NewCountService(repo, cache, unreadChat{}, nil, zap.NewNop())
				var manager tx.Manager = gormx.NewTxManager(db)
				failure := errors.New("admin commit failed")
				if stage == "commit" {
					manager = failedInboxCommit{manager, failure}
				} else {
					statement := `CREATE TRIGGER reject_admin_write BEFORE UPDATE ON user_global_notifications BEGIN SELECT RAISE(ABORT, 'admin update unavailable'); END`
					if action == "delete" {
						statement = `CREATE TRIGGER reject_admin_write BEFORE DELETE ON notification_user_reads BEGIN SELECT RAISE(ABORT, 'admin read deletion unavailable'); END`
					}
					if err := db.Exec(statement).Error; err != nil {
						t.Fatal(err)
					}
				}
				admin := app.NewAdminService(manager, repo, nil, counts)
				if action == "update" {
					err = admin.UpdateSystemNotice(t.Context(), app.UpdateSystemNoticeCommand{ID: notice.ID(), ReceiverID: 2, Title: "changed"})
					if !errors.Is(err, app.ErrNoticeUpdateFailed) {
						t.Fatalf("update classification changed: %v", err)
					}
				} else {
					err = admin.AdminDelete(t.Context(), []uint{notice.ID()})
				}
				if err == nil || stage == "commit" && !errors.Is(err, failure) {
					t.Fatalf("failure cause lost: %v", err)
				}
				current, findErr := repo.FindByID(t.Context(), notice.ID())
				if findErr != nil || current == nil || current.ReceiverID() != 1 || current.Title() != "original" || current.Type() != enum.NotificationApplyType {
					t.Fatalf("notice not rolled back: %+v %v", current, findErr)
				}
				reads, readErr := repo.ReadSet(t.Context(), 1, []uint{notice.ID()})
				if readErr != nil || len(reads) != 1 || len(cache.invalidated) != 0 {
					t.Fatalf("read/cache changed before commit: %v %v %v", reads, cache.invalidated, readErr)
				}
			})
		}
	}
}
