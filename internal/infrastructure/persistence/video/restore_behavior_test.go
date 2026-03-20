package video_test

import (
	"fmt"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"

	"go.uber.org/zap"
)

func restoreService(f *fixture) *app.AdminVideoService {
	return app.NewAdminVideoService(gormx.NewTxManager(f.db), f.videos, f.videos, nil, nil, nil,
		inactiveRevisionMeta{}, nil, nil, app.NewVectorSyncService(nil, nil, nil), nil, nil, nil, zap.NewNop())
}

func TestRestoreOnlyDeletedVideosAndIgnoresRepeatedIDs(t *testing.T) {
	f := newRevisionFixture(t)
	deleted := scheduleVideo(t, f, enum.VideoStatusDeleted)
	private := scheduleVideo(t, f, enum.VideoStatusPrivate)
	ids := []uint{private, deleted, deleted, 0, 999999}
	rows, err := restoreService(f).Restore(t.Context(), ids)
	if err != nil || rows != 1 {
		t.Fatalf("restore: rows=%d err=%v", rows, err)
	}
	requireScheduleStatus(t, f, deleted, enum.VideoStatusPublished)
	requireScheduleStatus(t, f, private, enum.VideoStatusPrivate)
	if ids[0] != private || ids[1] != deleted {
		t.Fatal("restoration mutated caller's ID order")
	}
	rows, err = restoreService(f).Restore(t.Context(), ids)
	if err != nil || rows != 0 {
		t.Fatalf("repeated restore: rows=%d err=%v", rows, err)
	}
}

func TestRestoreFailureRollsBackWholeBatch(t *testing.T) {
	f := newRevisionFixture(t)
	first := scheduleVideo(t, f, enum.VideoStatusDeleted)
	second := scheduleVideo(t, f, enum.VideoStatusDeleted)
	if err := f.db.Exec(fmt.Sprintf(`CREATE TRIGGER reject_second_restore BEFORE UPDATE OF status ON videos WHEN NEW.id = %d BEGIN SELECT RAISE(ABORT, 'restore unavailable'); END`, second)).Error; err != nil {
		t.Fatal(err)
	}
	if rows, err := restoreService(f).Restore(t.Context(), []uint{first, second}); err == nil || rows != 0 {
		t.Fatalf("failed batch must not report committed rows: rows=%d err=%v", rows, err)
	}
	requireScheduleStatus(t, f, first, enum.VideoStatusDeleted)
	requireScheduleStatus(t, f, second, enum.VideoStatusDeleted)
}
