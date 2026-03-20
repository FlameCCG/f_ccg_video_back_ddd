package comment

import (
	"context"
	"reflect"
	"testing"

	commententity "f_ccg_video_back_ddd/internal/domain/comment/entity"
	"go.uber.org/zap"
)

type deletePlanRepo struct {
	serviceRepo
	t          *testing.T
	decrements map[uint]int64
}

func (r *deletePlanRepo) transaction(ctx context.Context) {
	r.t.Helper()
	if ctx.Value(likeTxKey{}) != true {
		r.t.Fatal("comment write escaped transaction")
	}
}
func (r *deletePlanRepo) BriefByIDs(context.Context, []uint) ([]commententity.Comment, error) {
	root, otherRoot := uint(1), uint(3)
	return []commententity.Comment{{ID: 2, VideoID: 10, RootID: &root}, {ID: 1, VideoID: 10}, {ID: 4, DynamicID: 11, RootID: &otherRoot}}, nil
}
func (r *deletePlanRepo) ChildrenBrief(_ context.Context, roots []uint) ([]commententity.Comment, error) {
	if !reflect.DeepEqual(roots, []uint{1}) {
		r.t.Fatalf("roots=%v", roots)
	}
	root := uint(1)
	return []commententity.Comment{{ID: 2, VideoID: 10, RootID: &root}, {ID: 5, VideoID: 10, RootID: &root}}, nil
}
func (r *deletePlanRepo) DecrReplyCountBy(ctx context.Context, root uint, delta int64) error {
	r.transaction(ctx)
	if r.decrements == nil {
		r.decrements = map[uint]int64{}
	}
	r.decrements[root] += delta
	return nil
}
func (r *deletePlanRepo) DecrReplyCount(ctx context.Context, root uint) error {
	return r.DecrReplyCountBy(ctx, root, 1)
}
func (r *deletePlanRepo) DeleteByIDs(ctx context.Context, ids []uint) error {
	r.transaction(ctx)
	r.deleted = ids
	return nil
}
func (r *deletePlanRepo) DeleteByID(ctx context.Context, id uint) error {
	r.transaction(ctx)
	r.deleted = append(r.deleted, id)
	return nil
}

type deleteMetrics struct {
	t           *testing.T
	tx          *likeTx
	count, heat int64
}

func (m *deleteMetrics) UpdateCommentCount(_ context.Context, _ uint, delta int64) {
	if m.tx.active {
		m.t.Fatal("counter changed before commit")
	}
	m.count += delta
}
func (m *deleteMetrics) RecordHotScoreForComment(_ context.Context, _ uint, delta int64) {
	if m.tx.active {
		m.t.Fatal("heat changed before commit")
	}
	m.heat += delta
}

func TestAdminDeleteDeduplicatesOverlappingRootsAndReplies(t *testing.T) {
	txm := &likeTx{}
	repo := &deletePlanRepo{t: t}
	likes := &serviceLikes{}
	cache := &serviceCache{}
	metrics := &deleteMetrics{t: t, tx: txm}
	s := NewDeleteService(txm, repo, likes, serviceFacts{}, cache, metrics, metrics, zap.NewNop())
	count, err := s.AdminDelete(t.Context(), []uint{2, 1, 4})
	if err != nil || count != 4 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	want := []uint{2, 1, 4, 5}
	if !reflect.DeepEqual(repo.deleted, want) || !reflect.DeepEqual(likes.deleted, want) || !reflect.DeepEqual(cache.dropped, want) {
		t.Fatalf("delete=%v likes=%v cache=%v", repo.deleted, likes.deleted, cache.dropped)
	}
	if !reflect.DeepEqual(repo.decrements, map[uint]int64{3: 1}) || metrics.count != -3 || metrics.heat != -3 {
		t.Fatalf("reply deltas=%v video=%d heat=%d", repo.decrements, metrics.count, metrics.heat)
	}
}

func TestDeleteReplyDecrementsOnlyItsRootAndVideo(t *testing.T) {
	root := uint(1)
	txm := &likeTx{}
	repo := &deletePlanRepo{t: t, serviceRepo: serviceRepo{target: &commententity.Comment{ID: 2, VideoID: 10, UserID: 7, RootID: &root}}}
	likes := &serviceLikes{}
	cache := &serviceCache{}
	metrics := &deleteMetrics{t: t, tx: txm}
	s := NewDeleteService(txm, repo, likes, serviceFacts{}, cache, metrics, metrics, zap.NewNop())
	if err := s.Delete(t.Context(), 7, 2); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repo.decrements, map[uint]int64{1: 1}) || !reflect.DeepEqual(likes.deleted, []uint{2}) || metrics.count != -1 || metrics.heat != -1 {
		t.Fatalf("reply deltas=%v deleted=%v video=%d heat=%d", repo.decrements, likes.deleted, metrics.count, metrics.heat)
	}
}
