package video_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"
	"f_ccg_video_back_ddd/internal/infrastructure/storage"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestStoragePathStillReferencedRecognizesLegacyURLs(t *testing.T) {
	const source = "/user_files/hash/_source_hash.mp4"
	cases := []struct {
		name   string
		stored string
		want   bool
	}{
		{"relative", source, true},
		{"without_leading_slash", source[1:], true},
		{"legacy_url_with_bucket", "http://minio:9002/videos" + source, true},
		{"legacy_url_with_query", "https://old-host/videos" + source + "?download=1#source", true},
		{"escaped_url_path", "https://old-host/videos/user%5Ffiles/hash/_source_hash.mp4", true},
		{"different_hash", "/user_files/other/_source_other.mp4", false},
		{"same_filename_different_directory", "/other/hash/_source_hash.mp4", false},
		{"directory_boundary", "/other_user_files/hash/_source_hash.mp4", false},
		{"filename_suffix", source + ".backup", false},
		{"query_is_not_a_reference", "https://old-host/other.mp4?next=" + source, false},
		{"underscore_is_literal", "/user_files/hash/XsourceXhash.mp4", false},
	}
	for _, target := range []struct{ table, column string }{
		{"videos", "cover"},
		{"video_resources", "file_url"},
		{"video_part_resources", "file_url"},
	} {
		t.Run(target.table, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					db := newStorageReferenceDB(t)
					if err := db.Table(target.table).Create(map[string]any{target.column: tc.stored}).Error; err != nil {
						t.Fatal(err)
					}
					got, err := infra.NewVideoRepository(db).StoragePathStillReferenced(t.Context(), source)
					if err != nil || got != tc.want {
						t.Fatalf("reference %q: got %v, %v; want %v", tc.stored, got, err, tc.want)
					}
				})
			}
		})
	}
}

func newStorageReferenceDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "references.db") + "?_pragma=synchronous(OFF)&_pragma=journal_mode(MEMORY)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	// This query needs only the reference columns, without unrelated model hooks.
	for _, statement := range []string{
		"CREATE TABLE videos (cover TEXT)",
		"CREATE TABLE video_resources (file_url TEXT)",
		"CREATE TABLE video_part_resources (file_url TEXT)",
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

type storageReferenceGuard struct{}

func (storageReferenceGuard) Run(ctx context.Context, _ []string, fn func(context.Context) error) error {
	return fn(ctx)
}

func (storageReferenceGuard) TryRun(ctx context.Context, _ string, fn func(context.Context) error) (bool, error) {
	return true, fn(ctx)
}

func TestUploadCleanupPreservesLegacyReferencesAndDeletesOrphans(t *testing.T) {
	root := t.TempDir()
	db := newStorageReferenceDB(t)
	old := time.Now().Add(-3 * time.Hour)
	files := make(map[string]string)
	for _, hash := range []string{"video", "part", "orphan"} {
		file := filepath.Join(root, "videos", hash, "_source_"+hash+".mp4")
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("source"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(file, old, old); err != nil {
			t.Fatal(err)
		}
		files[hash] = file
	}
	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{Storage: &site.Storage{
		Local: &site.Local{Path: root},
	}}})
	scanner := storage.NewUploadGarbageScanner(provider, nil, nil)
	items, err := scanner.List(t.Context(), time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	if err != nil || len(items) != 3 {
		t.Fatalf("scan: got %d items, %v; want 3", len(items), err)
	}
	for _, item := range items {
		table := map[string]string{"video": "video_resources", "part": "video_part_resources"}[item.FileHash]
		if table == "" {
			continue
		}
		if err := db.Table(table).Create(map[string]any{"file_url": "https://legacy.example" + item.SourcePath}).Error; err != nil {
			t.Fatal(err)
		}
	}
	app.NewUploadCleanupService(scanner, infra.NewVideoRepository(db), storageReferenceGuard{}, provider, nil).Run(t.Context())
	for hash, file := range files {
		_, err := os.Stat(file)
		if hash == "orphan" {
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unreferenced source was not removed: %v", err)
			}
		} else if err != nil {
			t.Fatalf("source with legacy %s reference was removed: %v", hash, err)
		}
	}
}
