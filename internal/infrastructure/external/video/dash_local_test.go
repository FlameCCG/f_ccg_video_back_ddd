package video_test

import (
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	infra "f_ccg_video_back_ddd/internal/infrastructure/external/video"
	"f_ccg_video_back_ddd/internal/infrastructure/storage"
	"fmt"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"testing"
)

func TestStoredDASHResolvesRootRelativeManifestAndBatchesManySegments(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "videos", "dash")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `<MPD><Period><AdaptationSet contentType="video"><Representation id="0"><SegmentTemplate initialization="init.mp4" media="seg-$Number$.m4s"><SegmentTimeline><S d="2048" r="999"/></SegmentTimeline></SegmentTemplate></Representation></AdaptationSet></Period></MPD>`
	if err := os.WriteFile(filepath.Join(dir, "manifest.mpd"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"init.mp4": "init"}
	for number := 1; number <= 1000; number++ {
		files[fmt.Sprintf("seg-%d.m4s", number)] = "segment"
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{Storage: &site.Storage{Local: &site.Local{Enable: true, Path: root}}}})
	resolver := infra.NewStoredDASHVariantResolver(storage.NewResolver(provider, nil, nil, zap.NewNop()), zap.NewNop())
	variants, err := resolver.Variants(t.Context(), 1, "/videos/dash/manifest.mpd")
	if err != nil {
		t.Fatal(err)
	}
	if len(variants) != 1 || variants[0].FileSize != 7004 || variants[0].FileName != "init.mp4" {
		t.Fatalf("variants=%+v", variants)
	}
}
