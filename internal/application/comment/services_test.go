package comment

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	commentdomain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

type serviceRepo struct {
	commentdomain.Repository
	target   *commentdomain.Comment
	created  *commentdomain.Comment
	replies  uint
	deleted  []uint
	pinned   bool
	unpinned uint
}

func (r *serviceRepo) Create(_ context.Context, c *commentdomain.Comment) (*commentdomain.Comment, error) {
	snapshot := c.Snapshot()
	snapshot.ID = 50
	r.created = commentdomain.RestoreComment(snapshot)
	return r.created, nil
}
func (r *serviceRepo) IncrReplyCount(_ context.Context, id uint) error { r.replies = id; return nil }
func (r *serviceRepo) FindByID(context.Context, uint) (*commentdomain.Comment, error) {
	return r.target, nil
}
func (r *serviceRepo) FindPinTargetForUpdate(context.Context, uint) (*commentdomain.Comment, error) {
	return r.target, nil
}
func (r *serviceRepo) ChildIDs(context.Context, uint) ([]uint, error) { return []uint{2, 3}, nil }
func (r *serviceRepo) DeleteByRootID(context.Context, uint) error     { return nil }
func (r *serviceRepo) DeleteByID(_ context.Context, id uint) error {
	r.deleted = append(r.deleted, id)
	return nil
}
func (r *serviceRepo) UnpinByVideo(_ context.Context, id uint) error { r.unpinned = id; return nil }
func (r *serviceRepo) SetPinned(_ context.Context, _ uint, pinned bool, at *time.Time) error {
	r.pinned = pinned && at != nil
	return nil
}

type serviceLikes struct {
	commentdomain.LikeRepository
	deleted []uint
	users   []uint
}

func (r *serviceLikes) DeleteByCommentIDs(_ context.Context, ids []uint) error {
	r.deleted = ids
	return nil
}
func (r *serviceLikes) UserIDsByComment(context.Context, uint) ([]uint, error) { return r.users, nil }
func (r *serviceLikes) LikedCommentIDs(context.Context, uint, []uint) (map[uint]bool, error) {
	return map[uint]bool{1: true}, nil
}

type serviceFacts struct{ RelatedQuery }

func (serviceFacts) UserProfile(context.Context, uint) (*UserProfile, error) {
	return &UserProfile{ID: 7, Username: "actor"}, nil
}
func (serviceFacts) UserProfileByUsername(context.Context, string) (*UserProfile, error) {
	return &UserProfile{ID: 9}, nil
}
func (serviceFacts) PublishedVideoBrief(context.Context, uint) (*VideoBrief, error) {
	return &VideoBrief{ID: 10, AuthorUsername: "owner"}, nil
}
func (serviceFacts) DynamicBrief(context.Context, uint) (*DynamicBrief, error) {
	return &DynamicBrief{ID: 11, UserID: 9}, nil
}
func (serviceFacts) VideoAuthorRef(context.Context, uint) (*VideoAuthorRef, error) {
	return &VideoAuthorRef{AuthorUserID: 9}, nil
}

type serviceQuery struct {
	Query
	parent *commentdomain.ParentBrief
}

func (q serviceQuery) ParentBrief(context.Context, uint, uint, uint) (*commentdomain.ParentBrief, error) {
	return q.parent, nil
}

type serviceCounter struct{ delta int64 }

func (c *serviceCounter) UpdateCommentCount(_ context.Context, _ uint, delta int64) { c.delta += delta }

type serviceCache struct {
	LikeCache
	complete         bool
	cleared, written bool
	dropped          []uint
	counts           map[uint]int64
}

func (*serviceCache) Available() bool                                           { return true }
func (c *serviceCache) Complete(context.Context, uint) (bool, error)            { return c.complete, nil }
func (c *serviceCache) Clear(context.Context, uint) error                       { c.cleared = true; return nil }
func (c *serviceCache) Write(context.Context, uint, []uint) error               { c.written = true; return nil }
func (c *serviceCache) Drop(_ context.Context, id uint)                         { c.dropped = append(c.dropped, id) }
func (c *serviceCache) Counts(context.Context, []uint) (map[uint]int64, []uint) { return c.counts, nil }
func (*serviceCache) LikedFlags(context.Context, []uint, uint) (map[uint]bool, []uint) {
	return nil, []uint{1}
}
func (*serviceCache) ReplyCounts(context.Context, []uint) map[uint]int64 { return nil }

type serviceNotices struct {
	notices     []Notice
	invalidated []int
	live        bool
}

func (n *serviceNotices) CreateNotices(ctx context.Context, notices []Notice) error {
	_, deadline := ctx.Deadline()
	n.live = ctx.Err() == nil && deadline
	n.notices = append(n.notices, notices...)
	return nil
}
func (n *serviceNotices) InvalidateCount(id int) { n.invalidated = append(n.invalidated, id) }

func TestCreateServicesPreserveTargetsAndReplyRoots(t *testing.T) {
	for _, dynamic := range []bool{false, true} {
		for _, reply := range []bool{false, true} {
			r := &serviceRepo{}
			counter := &serviceCounter{}
			parentID, rootID := uint(20), uint(19)
			cmd := CreateCommand{UserID: 7, Content: "hello", VideoID: 10}
			if dynamic {
				cmd.VideoID = 0
				cmd.DynamicID = 11
			}
			if reply {
				cmd.ParentID = &parentID
			}
			q := serviceQuery{parent: &commentdomain.ParentBrief{UserID: 9, RootID: &rootID}}
			s := NewCreateService(&likeTx{}, r, q, serviceFacts{}, counter, nil, nil, NewNotificationDispatcher(r, serviceFacts{}, nil, nil, zap.NewNop()))
			item, err := s.Create(t.Context(), cmd)
			if err != nil || item.ID != 50 || r.created.VideoID() != cmd.VideoID || r.created.DynamicID() != cmd.DynamicID {
				t.Fatalf("target/reply %v/%v: item=%+v err=%v", dynamic, reply, item, err)
			}
			if reply && (r.replies != rootID || *item.RootID != rootID || item.ReplyTo.UserID != 9) {
				t.Fatalf("reply root lost: %+v", item)
			}
			if !reply && item.RootID != nil {
				t.Fatal("root comment became reply")
			}
			if (counter.delta == 1) == dynamic {
				t.Fatalf("video counter changed for dynamic=%v", dynamic)
			}
		}
	}
}

func TestDeleteServiceAuthorizationAndRootCascade(t *testing.T) {
	r := &serviceRepo{target: commentdomain.RestoreComment(commentdomain.Snapshot{ID: 1, UserID: 7, DynamicID: 11})}
	likes := &serviceLikes{}
	cache := &serviceCache{}
	s := NewDeleteService(&likeTx{}, r, likes, serviceFacts{}, cache, nil, nil, zap.NewNop())
	if err := s.Delete(t.Context(), 8, 1); !errors.Is(err, commentdomain.ErrDeleteForbidden) {
		t.Fatalf("unauthorized delete: %v", err)
	}
	if err := s.Delete(t.Context(), 9, 1); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(likes.deleted, []uint{1, 2, 3}) || !reflect.DeepEqual(cache.dropped, likes.deleted) {
		t.Fatalf("cascade: %v %v", likes.deleted, cache.dropped)
	}
}

func TestPinServiceAuthorAndReplacement(t *testing.T) {
	r := &serviceRepo{target: commentdomain.RestoreComment(commentdomain.Snapshot{ID: 1, VideoID: 10})}
	s := NewPinService(&likeTx{}, r, serviceFacts{})
	if err := s.Pin(t.Context(), 8, 1, true); !errors.Is(err, commentdomain.ErrPinForbidden) {
		t.Fatal(err)
	}
	if err := s.Pin(t.Context(), 9, 1, true); err != nil {
		t.Fatal(err)
	}
	if r.unpinned != 10 || !r.pinned {
		t.Fatal("pin did not replace within target")
	}
}

func TestLikeCacheCoordinatorPreservesZeroSeedDistinction(t *testing.T) {
	for _, users := range [][]uint{nil, {7}} {
		cache := &serviceCache{}
		s := NewLikeCacheCoordinator(&serviceLikes{users: users}, cache, nil, zap.NewNop())
		if err := s.ensureLikeCacheReady(t.Context(), 1); err != nil {
			t.Fatal(err)
		}
		if cache.written != (len(users) > 0) || cache.cleared != (len(users) == 0) {
			t.Fatal("hot path must clear empty partial keys without seeding count")
		}
		cache.written = false
		if err := s.rebuildLikeCache(t.Context(), 1); err != nil || !cache.written {
			t.Fatalf("read rebuild must seed even zero: %v", err)
		}
	}
}

func TestQueryProjectionPrefersCacheAndFallsBackForFlags(t *testing.T) {
	cache := &serviceCache{counts: map[uint]int64{1: 12}}
	s := NewQueryService(&serviceLikes{}, nil, nil, cache, NewLikeCacheCoordinator(&serviceLikes{}, cache, nil, zap.NewNop()))
	items := s.buildCommentItems(t.Context(), []CommentRow{{ID: 1, LikeCount: 2, ReplyCount: 3}}, 7)
	if len(items) != 1 || items[0].LikeCount != 12 || items[0].ReplyCount != 3 || !items[0].IsLiked {
		t.Fatalf("projection: %+v", items)
	}
}

func TestNotificationDispatcherDedupAndDetachedLifetime(t *testing.T) {
	n := &serviceNotices{}
	tasks := ctxutil.NewDetachedTaskGroup()
	s := NewNotificationDispatcher(nil, serviceFacts{}, n, tasks, zap.NewNop())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	s.scheduleCreateNotifications(ctx, UserProfile{ID: 7}, &VideoBrief{ID: 10, AuthorUsername: "owner"}, nil, commentdomain.RestoreComment(commentdomain.Snapshot{ID: 1}), &ReplyInfo{UserID: 9})
	if err := tasks.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !n.live || len(n.notices) != 1 || n.notices[0].Title != noticeTitleReply || !reflect.DeepEqual(n.invalidated, []int{9}) {
		t.Fatalf("dedup/lifetime: %+v", n)
	}
	if err := tasks.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	if tasks.Go(ctx, time.Second, func(context.Context) { t.Error("task accepted after stop") }) {
		t.Fatal("accepted after stop")
	}
}
