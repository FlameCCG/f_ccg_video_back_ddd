package notification_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/notification"
	"f_ccg_video_back_ddd/internal/domain/notification/entity"
	"f_ccg_video_back_ddd/internal/domain/notification/repository"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/notification"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type countCache struct {
	mu               sync.Mutex
	available, fresh bool
	data             map[int]app.NotificationCountSummary
	invalidated      []int
	all              int
}

func (c *countCache) Available() bool { return c.available }
func (c *countCache) Load(id int) (*app.NotificationCountSummary, bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.data[id]
	return &value, c.fresh, ok
}
func (c *countCache) Store(id int, value app.NotificationCountSummary) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.available {
		if c.data == nil {
			c.data = map[int]app.NotificationCountSummary{}
		}
		c.data[id] = value
		c.fresh = true
	}
}
func (c *countCache) Invalidate(id int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, id)
	c.invalidated = append(c.invalidated, id)
}
func (c *countCache) InvalidateAll() { c.mu.Lock(); defer c.mu.Unlock(); c.data = nil; c.all++ }

type countRepo struct {
	repository.Repository
	fail  error
	check func(context.Context)
	batch func()
}

func (r countRepo) CountUnreadGrouped(ctx context.Context, id int) (map[enum.Type]int64, error) {
	if r.check != nil {
		r.check(ctx)
	}
	return map[enum.Type]int64{enum.NotificationApplyType: int64(id), enum.NotificationVideoLikeType: 1, enum.NotificationLikeCommentType: 2, enum.NotificationLikeDynamicType: 3, enum.NotificationAtType: 4, enum.NotificationSystemType: 5}, r.fail
}
func (r countRepo) CreateBatch(context.Context, []entity.Notification) error {
	if r.batch != nil {
		r.batch()
	}
	return r.fail
}

type unreadChat struct{ fail error }

func (c unreadChat) TotalUnread(context.Context, uint) (int64, error) { return 7, c.fail }

func TestDeliveryInvalidatesOnlyAfterSuccessfulBatch(t *testing.T) {
	cache := &countCache{available: true}
	committed := false
	repo := countRepo{batch: func() {
		committed = true
		if len(cache.invalidated) != 0 {
			t.Error("invalidated before write")
		}
	}}
	counts := app.NewCountService(repo, cache, unreadChat{}, nil, zap.NewNop())
	delivery := app.NewDeliveryService(repo, counts)
	if err := delivery.CreateBatch(t.Context(), []entity.Notification{{ReceiverID: 1}, {ReceiverID: 1}, {ReceiverID: 2}}); err != nil {
		t.Fatal(err)
	}
	if !committed || !reflect.DeepEqual(cache.invalidated, []int{1, 2}) {
		t.Fatalf("invalidations=%v", cache.invalidated)
	}
	failure := errors.New("insert failed")
	delivery = app.NewDeliveryService(countRepo{fail: failure}, counts)
	if err := delivery.CreateBatch(t.Context(), []entity.Notification{{ReceiverID: 3}}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if len(cache.invalidated) != 2 {
		t.Fatal("failed batch invalidated cache")
	}
	counts.InvalidateCount(-1)
	if cache.all != 1 {
		t.Fatal("broadcast did not invalidate all")
	}
}

func TestCountSummaryCombinesUnreadAndDegradesWithoutCacheOrChat(t *testing.T) {
	for _, chatFailure := range []error{nil, errors.New("chat unavailable")} {
		counts := app.NewCountService(countRepo{}, &countCache{}, unreadChat{chatFailure}, nil, zap.NewNop())
		got, err := counts.CountSummary(t.Context(), 9)
		want := app.NotificationCountSummary{Reply: 9, Like: 6, At: 4, System: 5, Message: 7}
		if chatFailure != nil {
			want.Message = 0
		}
		if err != nil || *got != want {
			t.Fatalf("summary=%+v err=%v", got, err)
		}
	}
	failure := errors.New("database unavailable")
	counts := app.NewCountService(countRepo{fail: failure}, &countCache{}, unreadChat{}, nil, zap.NewNop())
	if _, err := counts.CountSummary(t.Context(), 9); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}

func TestStaleCountRefreshUsesManagedDetachedLifetime(t *testing.T) {
	tasks := ctxutil.NewDetachedTaskGroup()
	cache := &countCache{available: true, data: map[int]app.NotificationCountSummary{9: {Reply: 1}}}
	checked := make(chan bool, 1)
	repo := countRepo{check: func(ctx context.Context) { _, deadline := ctx.Deadline(); checked <- ctx.Err() == nil && deadline }}
	counts := app.NewCountService(repo, cache, unreadChat{}, tasks, zap.NewNop())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := counts.CountSummary(ctx, 9)
	if err != nil || got.Reply != 1 {
		t.Fatalf("stale summary=%+v err=%v", got, err)
	}
	if err := tasks.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !<-checked {
		t.Fatal("refresh inherited canceled request or lost timeout")
	}
	got, err = counts.CountSummary(t.Context(), 9)
	if err != nil || got.Reply != 9 {
		t.Fatalf("refreshed=%+v err=%v", got, err)
	}
	if err := tasks.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	cache.mu.Lock()
	cache.fresh = false
	cache.mu.Unlock()
	if _, err := counts.CountSummary(t.Context(), 9); err != nil {
		t.Fatal(err)
	}
	select {
	case <-checked:
		t.Fatal("refresh ran after shutdown")
	default:
	}
}

func TestInboxReceiverIsolationAndAdminReceiverChanges(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "inbox.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.AutoMigrate(&po.UserGlobalNotification{}, &po.NotificationUserRead{}); err != nil {
		t.Fatal(err)
	}
	repo := infra.NewRepository(db)
	cache := &countCache{available: true}
	counts := app.NewCountService(repo, cache, unreadChat{}, nil, zap.NewNop())
	delivery := app.NewDeliveryService(repo, counts)
	inbox := app.NewInboxService(repo, counts)
	admin := app.NewAdminService(repo, nil, counts)
	rows := []entity.Notification{{ReceiverID: 1, Type: enum.NotificationApplyType}, {ReceiverID: 2, Type: enum.NotificationApplyType}, {ReceiverID: -1, Type: enum.NotificationSystemType}}
	if err := delivery.CreateBatch(t.Context(), rows); err != nil {
		t.Fatal(err)
	}
	items, total, err := inbox.List(t.Context(), 1, 1, 20, "")
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("items=%+v total=%d err=%v", items, total, err)
	}
	ids := []uint{rows[0].ID, rows[1].ID, rows[2].ID}
	if err := inbox.MarkRead(t.Context(), 1, ids); err != nil {
		t.Fatal(err)
	}
	read1, err := repo.ReadSet(t.Context(), 1, ids)
	if err != nil {
		t.Fatal(err)
	}
	read2, err := repo.ReadSet(t.Context(), 2, ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(read1) != 2 || len(read2) != 0 {
		t.Fatalf("reads leaked: %v %v", read1, read2)
	}
	if _, leaked := read1[rows[1].ID]; leaked {
		t.Fatal("marked other receiver's notice")
	}
	if err := inbox.Delete(t.Context(), 1, ids[:2]); err != nil {
		t.Fatal(err)
	}
	other, err := repo.FindByID(t.Context(), rows[1].ID)
	if err != nil || other == nil {
		t.Fatalf("deleted other receiver: %v", err)
	}
	if err := inbox.Delete(t.Context(), 1, ids[2:]); !errors.Is(err, entity.ErrSystemNotificationNotDeletable) {
		t.Fatal(err)
	}
	cache.invalidated = nil
	if err := admin.UpdateSystemNotice(t.Context(), app.UpdateSystemNoticeCommand{ID: rows[1].ID, ReceiverID: 1, Title: "moved"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cache.invalidated, []int{2, 1}) {
		t.Fatalf("old/new receivers=%v", cache.invalidated)
	}
}
