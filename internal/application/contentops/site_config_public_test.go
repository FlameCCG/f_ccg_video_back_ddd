package contentops

import (
	"f_ccg_video_back_ddd/internal/config/site"
	"testing"
)

func TestPublicSiteClampsChunkSizeToMaxChunkSize(t *testing.T) {
	t.Parallel()

	src := site.Site{Storage: &site.Storage{ChunkSize: 30, MaxChunkSize: 20, MaxFileSize: 700}}

	got := cloneSiteConfig(src)
	if got.Storage == nil || got.Storage.ChunkSize != 20 {
		t.Fatalf("public chunkSize = %+v, want 20", got.Storage)
	}

	if src.Storage.ChunkSize != 30 {
		t.Fatal("clamping public config mutated the source")
	}

	admin := cloneAdminSiteConfig(&src)
	if admin == nil || admin.Storage == nil || admin.Storage.ChunkSize != 30 {
		t.Fatalf("admin chunkSize = %+v, want the configured 30", admin)
	}
}
