package video

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

type securityVideoRepo struct {
	domain.VideoRepository
	author uint
}

func (r securityVideoRepo) PublishedAuthorIDByVideoID(context.Context, uint) (uint, error) {
	return r.author, nil
}

type securityLikes struct{ domain.LikeRepository }

func (securityLikes) Create(context.Context, uint, uint) error { return domain.ErrDuplicateLike }

type securityCoins struct{ domain.CoinRepository }

func (securityCoins) LockByVideoAndUser(context.Context, uint, uint) error { return nil }

type securityFolders struct{ domain.FolderRepository }

func (securityFolders) UserFavorited(context.Context, uint, uint) (bool, error) { return false, nil }
func (securityFolders) EnsureLink(context.Context, uint, uint) (bool, error)    { return true, nil }

type securityLedger struct {
	CoinLedger
	keys    map[string]bool
	rewards int
}

func (*securityLedger) LockAccounts(context.Context, []uint) error { return nil }
func (l *securityLedger) ApplyChange(_ context.Context, _ uint, _ float64, _ string, key *string) error {
	if key == nil {
		return errors.New("missing dedup key")
	}
	if !l.keys[*key] {
		l.keys[*key] = true
		l.rewards++
	}
	return nil
}

func TestTripleFavoriteCyclesShareLifetimeRewardKeyAndExcludeAuthor(t *testing.T) {
	ledger := &securityLedger{keys: map[string]bool{}}
	s := &InteractionService{videos: securityVideoRepo{author: 1}, likes: securityLikes{}, coins: securityCoins{}, folders: securityFolders{}, ledger: ledger}
	for i := 0; i < 100; i++ {
		if _, err := s.applyTriple(t.Context(), 10, 2, 1, 1); err != nil {
			t.Fatal(err)
		}
	}
	if ledger.rewards != 1 || !ledger.keys["favorite:2:10"] {
		t.Fatalf("reward cycles=%d keys=%v", ledger.rewards, ledger.keys)
	}
	if _, err := s.applyTriple(t.Context(), 10, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if ledger.rewards != 1 {
		t.Fatal("author received self-favorite reward")
	}
}

type failedDeleteRepo struct {
	domain.VideoRepository
	db      *gorm.DB
	failure error
}

func (r failedDeleteRepo) UpdateStatusBatch(ctx context.Context, _ []uint, status enum.VideoStatus) (int64, error) {
	result := gormx.DB(ctx, r.db).Exec("UPDATE videos SET status=?", status)
	return result.RowsAffected, result.Error
}
func (r failedDeleteRepo) HardDelete(ctx context.Context, _ []uint) (*domain.HardDeleteResult, []domain.StorageDeleteTarget, error) {
	if err := gormx.DB(ctx, r.db).Exec("DELETE FROM videos").Error; err != nil {
		return nil, nil, err
	}
	return nil, nil, r.failure
}

type deleteComments struct {
	CommentPurger
	db *gorm.DB
}

func (r deleteComments) PurgeByVideoIDs(ctx context.Context, _ []uint) (int64, int64, error) {
	res := gormx.DB(ctx, r.db).Exec("DELETE FROM comments")
	return res.RowsAffected, 0, res.Error
}

type deleteSchedule struct{ ScheduleStore }

func (deleteSchedule) Remove(context.Context, uint) error { return nil }

func TestHardDeleteFailureRollsBackStatusCommentsAndVideo(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "delete.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })
	for _, sql := range []string{"CREATE TABLE videos(id integer,status integer)", "CREATE TABLE comments(id integer)", "INSERT INTO videos VALUES(1,1)", "INSERT INTO comments VALUES(1)"} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	failure := errors.New("failure after dependent deletes")
	s := &AdminVideoService{txm: gormx.NewTxManager(db), videos: failedDeleteRepo{db: db, failure: failure}, comments: deleteComments{db: db}, schedule: deleteSchedule{}}
	if _, err := s.hardDelete(t.Context(), []uint{1}); !errors.Is(err, failure) {
		t.Fatalf("error=%v", err)
	}
	var status int
	db.Raw("SELECT status FROM videos WHERE id=1").Scan(&status)
	if status != 1 {
		t.Fatalf("status=%d", status)
	}
	var count int64
	db.Table("comments").Count(&count)
	if count != 1 {
		t.Fatalf("comments=%d", count)
	}
}

type deniedContent struct {
	err   error
	calls int
}

func (a *deniedContent) EnsureCanPublish(context.Context, uint) error { a.calls++; return a.err }
func TestDanmuChecksLiveAccountBeforeAnyWrite(t *testing.T) {
	denied := &deniedContent{err: errors.New("account banned")}
	s := &DanmuService{authorizer: denied}
	for i := 0; i < 2; i++ {
		if _, err := s.Send(t.Context(), 1, SendDanmuParams{VideoID: 1, Content: "hello"}); !errors.Is(err, denied.err) {
			t.Fatal(err)
		}
	}
	if denied.calls != 2 {
		t.Fatal("account check not repeated for each message")
	}
}
