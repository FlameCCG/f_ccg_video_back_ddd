package search_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/search"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 迁移自原 service/search_service/suggest_test.go 的两个 sqlite 回退用例。
//
// 原测试通过 global.DB 注入句柄；新实现由构造函数注入 *gorm.DB，测试直接建仓储。

// newFallbackDB 用 sqlite 临时库建出回退查询用到的四张表。
func newFallbackDB(t *testing.T) *gorm.DB {
	t.Helper()

	// synchronous(OFF) 让建表 DDL 免去 fsync，AutoMigrate 明显更快。
	dsn := filepath.Join(t.TempDir(), "search-fallback-test.db") +
		"?_pragma=synchronous(OFF)&_pragma=journal_mode(MEMORY)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		TranslateError:         true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	// Windows 上不关句柄会导致 t.TempDir 清理失败（文件仍被占用）。
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	if err := db.AutoMigrate(&po.Video{}, &po.User{}, &po.Tag{}, &po.UserFocus{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	return db
}

// seedVideos 跳过 GORM 钩子直接插入视频行（避免触发其它上下文的副作用）。
func seedVideos(t *testing.T, db *gorm.DB, videos ...po.Video) {
	t.Helper()

	session := db.Session(&gorm.Session{SkipHooks: true})

	for i := range videos {
		if err := session.Create(&videos[i]).Error; err != nil {
			t.Fatalf("create video failed: %v", err)
		}
	}
}

// TestVideoTitlesGroupsByTitleAndPublishedStatus 只取已发布视频，
// 标题去重（GROUP BY），前缀命中优先、其次按最新 id。
func TestVideoTitlesGroupsByTitleAndPublishedStatus(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedVideos(t, db,
		po.Video{Model: po.Model{ID: 1}, Title: "w draft", Status: enum.VideoStatusReview},
		po.Video{Model: po.Model{ID: 2}, Title: "w old", Status: enum.VideoStatusPublished},
		po.Video{Model: po.Model{ID: 3}, Title: "w duplicate", Status: enum.VideoStatusPublished},
		po.Video{Model: po.Model{ID: 4}, Title: "w duplicate", Status: enum.VideoStatusPublished},
		po.Video{Model: po.Model{ID: 5}, Title: "x ignored", Status: enum.VideoStatusPublished},
	)

	titles, err := infra.NewSuggestFallbackQuery(db).VideoTitles(context.Background(), "w", 10)
	if err != nil {
		t.Fatalf("VideoTitles: %v", err)
	}

	want := []string{"w duplicate", "w old"}
	if !reflect.DeepEqual(titles, want) {
		t.Fatalf("expected %v, got %v", want, titles)
	}
}

// TestTagNamesRequiresPublishedVideo 标签只在挂到已发布视频上时才进入联想。
func TestTagNamesRequiresPublishedVideo(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedVideos(t, db,
		po.Video{Model: po.Model{ID: 1}, Title: "w published", Status: enum.VideoStatusPublished},
		po.Video{Model: po.Model{ID: 2}, Title: "w review", Status: enum.VideoStatusReview},
	)

	session := db.Session(&gorm.Session{SkipHooks: true})
	for _, stmt := range []string{
		`INSERT INTO tags (id, name) VALUES (1, 'workflow')`,
		`INSERT INTO tags (id, name) VALUES (2, 'waiting')`,
		`INSERT INTO video_tags (video_id, tag_id) VALUES (1, 1)`,
		`INSERT INTO video_tags (video_id, tag_id) VALUES (2, 2)`,
	} {
		if err := session.Exec(stmt).Error; err != nil {
			t.Fatalf("seed fallback tags failed: %v", err)
		}
	}

	names, err := infra.NewSuggestFallbackQuery(db).TagNames(context.Background(), "w", 10)
	if err != nil {
		t.Fatalf("TagNames: %v", err)
	}

	want := []string{"workflow"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("expected %v, got %v", want, names)
	}
}

// TestUsernamesPrefersPrefixMatches 用户名联想按前缀命中优先，其次按最新 id。
func TestUsernamesPrefersPrefixMatches(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	session := db.Session(&gorm.Session{SkipHooks: true})

	users := []po.User{
		{Model: po.Model{ID: 1}, Username: "xwang", Email: "1@e.com", Password: "x"},
		{Model: po.Model{ID: 2}, Username: "wang", Email: "2@e.com", Password: "x"},
		{Model: po.Model{ID: 3}, Username: "bob", Email: "3@e.com", Password: "x"},
	}
	for i := range users {
		if err := session.Create(&users[i]).Error; err != nil {
			t.Fatalf("create user failed: %v", err)
		}
	}

	names, err := infra.NewSuggestFallbackQuery(db).Usernames(context.Background(), "w", 10)
	if err != nil {
		t.Fatalf("Usernames: %v", err)
	}

	// 前缀命中的 "wang" 排在中间片段命中的 "xwang" 之前。
	want := []string{"wang", "xwang"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("expected %v, got %v", want, names)
	}
}

// TestSuggestFallbackDegradesWithoutDB db 为 nil 或 size<=0 时返回空切片、不报错，
// 与原 global.DB == nil 的降级行为一致。
func TestSuggestFallbackDegradesWithoutDB(t *testing.T) {
	t.Parallel()

	offline := infra.NewSuggestFallbackQuery(nil)
	if offline.Available() {
		t.Fatal("nil db must report unavailable")
	}

	ctx := context.Background()
	for name, call := range map[string]func() ([]string, error){
		"titles":    func() ([]string, error) { return offline.VideoTitles(ctx, "w", 10) },
		"usernames": func() ([]string, error) { return offline.Usernames(ctx, "w", 10) },
		"tags":      func() ([]string, error) { return offline.TagNames(ctx, "w", 10) },
	} {
		got, err := call()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if got == nil || len(got) != 0 {
			t.Fatalf("%s = %v, want empty slice", name, got)
		}
	}

	live := infra.NewSuggestFallbackQuery(newFallbackDB(t))
	if got, err := live.VideoTitles(ctx, "w", 0); err != nil || len(got) != 0 {
		t.Fatalf("size<=0: got %v, err %v", got, err)
	}
}
