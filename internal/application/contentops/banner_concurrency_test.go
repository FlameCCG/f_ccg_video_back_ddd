package contentops

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	contentopsdomain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

type bannerTransactionKey struct{}

type serializedBannerTx struct {
	mu       sync.Mutex
	attempts chan struct{}
}

func (m *serializedBannerTx) Do(ctx context.Context, fn func(context.Context) error) error {
	m.attempts <- struct{}{}
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(context.WithValue(ctx, bannerTransactionKey{}, true))
}

type concurrentBannerRepo struct {
	fakeBannerRepo
	mu  sync.Mutex
	row contentopsdomain.BannerSnapshot
}

func (r *concurrentBannerRepo) FindByID(context.Context, uint) (*contentopsdomain.Banner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return contentopsdomain.RestoreBanner(r.row), nil
}

func (r *concurrentBannerRepo) FindByIDForUpdate(ctx context.Context, id uint) (*contentopsdomain.Banner, error) {
	if ctx.Value(bannerTransactionKey{}) != true {
		return nil, errors.New("placement read escaped transaction")
	}
	return r.FindByID(ctx, id)
}

func (r *concurrentBannerRepo) Update(ctx context.Context, _ uint, changes contentopsdomain.BannerChanges) error {
	if ctx.Value(bannerTransactionKey{}) != true {
		return errors.New("placement write escaped transaction")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if changes.Type != nil {
		r.row.Type = *changes.Type
	}
	if changes.PartitionID != nil {
		r.row.PartitionID = *changes.PartitionID
	}
	return nil
}

type pausedBannerPartition struct {
	read    chan struct{}
	proceed chan struct{}
}

func (q *pausedBannerPartition) Exists(ctx context.Context, _ uint) (bool, error) {
	close(q.read)
	select {
	case <-q.proceed:
		return true, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func TestBannerPlacementConcurrentUpdatesKeepTypeAndPartitionConsistent(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	repo := &concurrentBannerRepo{row: contentopsdomain.BannerSnapshot{
		ID: 1, Type: enum.BannerTypeHome, PartitionID: 3,
	}}
	txm := &serializedBannerTx{attempts: make(chan struct{}, 2)}
	partitions := &pausedBannerPartition{read: make(chan struct{}), proceed: make(chan struct{})}
	var resume sync.Once
	defer resume.Do(func() { close(partitions.proceed) })
	svc := NewBannerService(txm, repo, nil, partitions)
	await := func(ch <-chan struct{}) {
		t.Helper()
		select {
		case <-ch:
		case <-ctx.Done():
			t.Fatal("placement update did not reach expected interleaving")
		}
	}

	partitionID := uint(4)
	partitionDone := make(chan error, 1)
	go func() { partitionDone <- svc.Update(ctx, UpdateBannerParams{ID: 1, PartitionID: &partitionID}) }()
	await(partitions.read)
	await(txm.attempts)

	nonHome := int(enum.BannerTypeTop)
	typeDone := make(chan error, 1)
	go func() { typeDone <- svc.Update(ctx, UpdateBannerParams{ID: 1, Type: &nonHome}) }()
	await(txm.attempts)
	resume.Do(func() { close(partitions.proceed) })
	for _, done := range []<-chan error{partitionDone, typeDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("concurrent placement update did not finish")
		}
	}
	current, _ := repo.FindByID(ctx, 1)
	if current.Type() != enum.BannerTypeTop || current.PartitionID() != 0 {
		t.Fatalf("inconsistent placement: type=%d partitionID=%d", current.Type(), current.PartitionID())
	}

	// 仅修改分区时必须读取已切换的轮播类型，错误分类与响应文案遵守接口契约。
	err := svc.Update(ctx, UpdateBannerParams{ID: 1, PartitionID: &partitionID})
	var resolveErr *PartitionResolveError
	if !errors.As(err, &resolveErr) || resolveErr.Error() != "只有类型1支持设置分区ID" {
		t.Fatalf("latest non-home placement should reject a partition: %v", err)
	}
	current, _ = repo.FindByID(ctx, 1)
	if current.Type() != enum.BannerTypeTop || current.PartitionID() != 0 {
		t.Fatal("rejected placement changed persistent state")
	}
}

type bannerTransactionFunc func(context.Context, func(context.Context) error) error

func (fn bannerTransactionFunc) Do(ctx context.Context, action func(context.Context) error) error {
	return fn(ctx, action)
}

func TestBannerPlacementTransactionFailuresPreserveErrorsAndCache(t *testing.T) {
	failure := errors.New("transaction unavailable")
	for _, stage := range []string{"begin", "commit"} {
		t.Run(stage, func(t *testing.T) {
			svc, repo, cache, _ := newBannerServiceFixture()
			repo.byID[1] = contentopsdomain.RestoreBanner(contentopsdomain.BannerSnapshot{
				ID: 1, Type: enum.BannerTypeHome, PartitionID: 3,
			})
			svc.txm = bannerTransactionFunc(func(ctx context.Context, action func(context.Context) error) error {
				if stage == "commit" {
					if err := action(ctx); err != nil {
						return err
					}
				}
				return failure
			})
			nonHome := int(enum.BannerTypeTop)
			err := svc.Update(t.Context(), UpdateBannerParams{ID: 1, Type: &nonHome})
			if !errors.Is(err, failure) || errors.Is(err, ErrBannerNotFound) != (stage == "begin") {
				t.Fatalf("unexpected %s failure: %v", stage, err)
			}
			if cache.invalidCount != 0 {
				t.Fatal("transaction failure invalidated visible cache")
			}
			if repo.byID[1].Type() != enum.BannerTypeHome || repo.byID[1].PartitionID() != 3 {
				t.Fatal("failed transaction mutated repository-owned snapshot")
			}
			if stage == "begin" && repo.updatedID != 0 {
				t.Fatal("failed transaction start still wrote placement")
			}
		})
	}
}
