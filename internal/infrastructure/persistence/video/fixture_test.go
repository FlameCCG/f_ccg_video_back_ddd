package video_test

import (
	"path/filepath"
	"testing"

	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testAuthorAlice 共享测试作者用户名（原 test_fixtures_test.go 的同名常量）。
const testAuthorAlice = "alice"

// fixture 一套用 sqlite 临时库装配的真实仓储与查询（测试不走 wire）。
type fixture struct {
	db          *gorm.DB
	videos      *infra.VideoRepository
	danmus      *infra.DanmuRepository
	likes       *infra.LikeRepository
	coins       *infra.CoinRepository
	folders     *infra.FolderRepository
	history     *infra.HistoryRepository
	partitions  *infra.PartitionRepository
	reports     *infra.ReportRepository
	detail      *infra.DetailQuery
	feed        *infra.FeedQuery
	recommend   *infra.RecommendQuery
	danmuQuery  *infra.DanmuQuery
	folderQuery *infra.FolderQuery
	historyList *infra.HistoryQuery
	adminVideos *infra.AdminVideoQuery
	reportList  *infra.ReportQuery
	userVideos  *infra.UserVideoQuery
	notify      *infra.NotifySourceQuery
	rename      *infra.AuthorRenameRepository
}

// newFixture 建库、建表并装配全部仓储/查询。
func newFixture(t *testing.T) *fixture {
	t.Helper()

	// synchronous(OFF) 让建表 DDL 免去 fsync，AutoMigrate 明显更快。
	dsn := filepath.Join(t.TempDir(), "video-test.db") +
		"?_pragma=synchronous(OFF)&_pragma=journal_mode(MEMORY)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		TranslateError:         true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	if err := db.AutoMigrate(
		&po.User{}, &po.Video{}, &po.VideoPartition{}, &po.VideoPart{},
		&po.VideoResource{}, &po.VideoPartResource{}, &po.Tag{},
		&po.VideoLike{}, &po.VideoCoin{}, &po.VideoPlayHistory{},
		&po.FavoriteFolder{}, &po.FavoriteFolderVideo{},
		&po.Danmu{}, &po.DanmuLike{},
		&po.VideoReport{}, &po.DanmuReport{},
		&po.Comment{}, &po.CommentLike{},
	); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}

	t.Cleanup(func() { _ = sqlDB.Close() })

	return &fixture{
		db:          db,
		videos:      infra.NewVideoRepository(db),
		danmus:      infra.NewDanmuRepository(db),
		likes:       infra.NewLikeRepository(db),
		coins:       infra.NewCoinRepository(db),
		folders:     infra.NewFolderRepository(db),
		history:     infra.NewHistoryRepository(db),
		partitions:  infra.NewPartitionRepository(db),
		reports:     infra.NewReportRepository(db),
		detail:      infra.NewDetailQuery(db),
		feed:        infra.NewFeedQuery(db),
		recommend:   infra.NewRecommendQuery(db),
		danmuQuery:  infra.NewDanmuQuery(db),
		folderQuery: infra.NewFolderQuery(db),
		historyList: infra.NewHistoryQuery(db),
		adminVideos: infra.NewAdminVideoQuery(db),
		reportList:  infra.NewReportQuery(db),
		userVideos:  infra.NewUserVideoQuery(db),
		notify:      infra.NewNotifySourceQuery(db),
		rename:      infra.NewAuthorRenameRepository(db),
	}
}

// session 返回跳过 GORM 钩子的会话（po.User 的 AfterCreate 会写 user_confs / 默认收藏夹）。
func (f *fixture) session() *gorm.DB {
	return f.db.Session(&gorm.Session{SkipHooks: true})
}

// createUsers 建用户（跳过钩子）。
func (f *fixture) createUsers(t *testing.T, users ...po.User) {
	t.Helper()

	session := f.session()

	for i := range users {
		if err := session.Create(&users[i]).Error; err != nil {
			t.Fatalf("create user failed: %v", err)
		}
	}
}
