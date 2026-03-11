package comment

import (
	"context"
	domain "f_ccg_video_back_ddd/internal/domain/comment"
	"testing"
)

type likeTxKey struct{}
type likeTx struct{ active bool }

func (m *likeTx) Do(ctx context.Context, fn func(context.Context) error) error {
	m.active = true
	defer func() { m.active = false }()
	return fn(context.WithValue(ctx, likeTxKey{}, true))
}

type likeCountRepo struct {
	domain.Repository
	t     *testing.T
	tx    *likeTx
	count int64
}

func (r *likeCountRepo) check(ctx context.Context) {
	r.t.Helper()
	if !r.tx.active || ctx.Value(likeTxKey{}) != true {
		r.t.Fatal("count accessed outside transaction")
	}
}
func (r *likeCountRepo) IncrLikeCount(ctx context.Context, _ uint) error {
	r.check(ctx)
	r.count++
	return nil
}
func (r *likeCountRepo) DecrLikeCount(ctx context.Context, _ uint) error {
	r.check(ctx)
	r.count--
	return nil
}
func (r *likeCountRepo) LikeCountByID(ctx context.Context, _ uint) (int64, error) {
	r.check(ctx)
	return r.count, nil
}

type conflictLikes struct {
	domain.LikeRepository
	t                           *testing.T
	duplicate, created, deleted bool
}

func (r *conflictLikes) Create(ctx context.Context, _ *domain.Like) error {
	if ctx.Value(likeTxKey{}) != true {
		r.t.Fatal("insert outside transaction")
	}
	r.created = true
	if r.duplicate {
		return domain.ErrLikeDuplicated
	}
	return nil
}
func (r *conflictLikes) Find(ctx context.Context, _, _ uint) (*domain.Like, error) {
	if !r.created {
		r.t.Fatal("check-before-insert race returned")
	}
	return &domain.Like{ID: 1}, nil
}
func (r *conflictLikes) DeleteByID(context.Context, uint) error { r.deleted = true; return nil }

func TestLikeFallbackInsertsBeforeCheckingAndReadsCountInTransaction(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		txm := &likeTx{}
		comments := &likeCountRepo{t: t, tx: txm}
		if duplicate {
			comments.count = 1
		}
		likes := &conflictLikes{t: t, duplicate: duplicate}
		svc := &Service{txm: txm, comments: comments, likes: likes}
		count, liked, err := svc.toggleLikeWithDB(context.Background(), 2, 3)
		if err != nil || liked == duplicate || count != comments.count || likes.deleted != duplicate {
			t.Fatalf("duplicate=%v count=%d liked=%v deleted=%v err=%v", duplicate, count, liked, likes.deleted, err)
		}
	}
}
