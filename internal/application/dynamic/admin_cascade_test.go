package dynamic_test

import (
	"context"
	app "f_ccg_video_back_ddd/internal/application/dynamic"
	commentrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/comment"
	dynamicrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/dynamic"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestAdminDeleteCascadesAtomically(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "delete failure"
		}
		t.Run(name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cascade.db")), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			defer sqlDB.Close()
			statements := []string{
				"CREATE TABLE dynamics (id INTEGER PRIMARY KEY)",
				"CREATE TABLE comments (id INTEGER PRIMARY KEY,dynamic_id INTEGER)",
				"CREATE TABLE comment_likes (id INTEGER PRIMARY KEY,comment_id INTEGER)",
				"INSERT INTO dynamics VALUES (1),(2)",
				"INSERT INTO comments VALUES (10,1),(20,2)",
				"INSERT INTO comment_likes VALUES (100,10),(200,20)",
			}
			if fail {
				statements = append(statements, "CREATE TRIGGER reject_delete BEFORE DELETE ON dynamics BEGIN SELECT RAISE(ABORT,'injected failure'); END")
			}
			for _, sql := range statements {
				if err := db.Exec(sql).Error; err != nil {
					t.Fatal(err)
				}
			}
			svc := app.NewAdminService(dynamicrepo.NewDynamicRepository(db), nil, gormx.NewTxManager(db), commentrepo.NewPurgeRepository(db))
			count, err := svc.Delete(context.Background(), []uint{1})
			if fail && err == nil {
				t.Fatal("expected delete failure")
			}
			if !fail && (err != nil || count != 1) {
				t.Fatalf("count=%d err=%v", count, err)
			}
			want := int64(1)
			if fail {
				want = 2
			}
			for _, table := range []string{"dynamics", "comments", "comment_likes"} {
				var count int64
				if err := db.Table(table).Count(&count).Error; err != nil {
					t.Fatal(err)
				}
				if count != want {
					t.Fatalf("%s rows=%d want=%d", table, count, want)
				}
			}
		})
	}
}
