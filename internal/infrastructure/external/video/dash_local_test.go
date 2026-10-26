package video_test

import (
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/infrastructure/storage"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	infra "f_ccg_video_back_ddd/internal/infrastructure/external/video"

	"go.uber.org/zap"
)

func TestStoredDASHResolvesRootRelativeManifestAndBatchesManySegments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	dir := filepath.Join(root, "videos", "dash")
	{
		err := os.MkdirAll(dir, 0o750)
		require.NoError(t, err)
	}

	manifest := `<MPD><Period><AdaptationSet contentType="video"><Representation id="0"><SegmentTemplate initialization="init.mp4" media="seg-$Number$.m4s"><SegmentTimeline><S d="2048" r="999"/></SegmentTimeline></SegmentTemplate></Representation></AdaptationSet></Period></MPD>`
	{
		err := os.WriteFile(filepath.Join(dir, "manifest.mpd"), []byte(manifest), 0o600)
		require.NoError(t, err)
	}

	files := map[string]string{"init.mp4": "init"}
	for number := 1; number <= 1000; number++ {
		files[fmt.Sprintf("seg-%d.m4s", number)] = "segment"
	}

	for name, data := range files {
		{
			err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600)
			require.NoError(t, err)
		}
	}

	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{Storage: &site.Storage{Local: &site.Local{Enable: true, Path: root}}}})
	resolver := infra.NewStoredDASHVariantResolver(storage.NewResolver(provider, nil, nil, zap.NewNop()), zap.NewNop())

	variants, err := resolver.Variants(t.Context(), 1, "/videos/dash/manifest.mpd")
	require.NoError(t, err)

	require.Len(t, variants, 1, "variants=%+v", variants)
	require.EqualValues(t, 7004, variants[0].FileSize, "variants=%+v", variants)
	require.Equal(t, "init.mp4", variants[0].FileName, "variants=%+v", variants)
}
