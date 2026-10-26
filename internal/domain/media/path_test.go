package media

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testMp4360Standard     = "mp4:360:standard"
	testSrc                = "src"
	testVideosHashX720pMp4 = "videos/hash/x_720p.mp4"
)

func TestStoredPathMatchesRelative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		stored   string
		relative string
		want     bool
	}{
		{
			name:     "minio url with bucket prefix",
			stored:   "http://minio:9000/bucket/videos/hash/x_720p.mp4",
			relative: testVideosHashX720pMp4,
			want:     true,
		},
		{
			name:     "plain relative path",
			stored:   testVideosHashX720pMp4,
			relative: testVideosHashX720pMp4,
			want:     true,
		},
		{
			name:     "different object",
			stored:   "http://minio:9000/bucket/videos/other/x_720p.mp4",
			relative: testVideosHashX720pMp4,
			want:     false,
		},
		{
			name:     "basename-only partial match rejected",
			stored:   "videos/hash/xx_720p.mp4",
			relative: "x_720p.mp4",
			want:     false,
		},
		{
			name:     "empty stored",
			stored:   "",
			relative: testVideosHashX720pMp4,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			{
				got := StoredPathMatchesRelative(tt.stored, tt.relative)
				require.Equal(t, tt.want, got, "StoredPathMatchesRelative(%q, %q) = %v, want %v", tt.stored, tt.relative, got, tt.want)
			}
		})
	}
}

// TestShouldDeleteOriginalSource 覆盖收尾时是否删除原始源对象的判定。
func TestShouldDeleteOriginalSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		initial string
		current string
		want    bool
	}{
		{name: "empty initial keeps source", initial: "", current: "videos/h/_normalized_h.mp4"},
		{name: "empty current means replaced", initial: "videos/h/_source_h.mp4", current: "", want: true},
		{
			name:    "different object means replaced",
			initial: "http://minio:9000/bucket/videos/h/_source_h.mp4",
			current: "http://minio:9000/bucket/videos/h/_normalized_h.mp4",
			want:    true,
		},
		{
			name:    "same object keeps source",
			initial: "http://minio:9000/bucket/videos/h/_source_h.mp4",
			current: "http://minio:9000/bucket/videos/h/_source_h.mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			{
				got := ShouldDeleteOriginalSource(tt.initial, tt.current)
				require.Equal(t, tt.want, got, "ShouldDeleteOriginalSource(%q, %q) = %v, want %v", tt.initial, tt.current, got, tt.want)
			}
		})
	}
}

// TestStoredPathBase 覆盖相对反斜杠路径与完整 URL 两种输入形态。
//
// 注意 `D:\tmp\a.mp4` 这类带盘符的绝对路径会被 url.Parse 当成 scheme "d" 解析，
// 取 u.Path 得到空串，最终返回 "."。这是原实现的既有行为（见 report.md FIND 条目），
// 此处逐字保留：转码产物路径只会是相对对象 key 或 http(s) URL，不会出现盘符路径。
func TestStoredPathBase(t *testing.T) {
	t.Parallel()

	{
		got := StoredPathBase(`videos\h\x.mp4`)
		require.Equal(t, "x.mp4", got, "relative backslash base = %q, want x.mp4", got)
	}

	{
		got := StoredPathBase("http://minio:9000/bucket/videos/h/x.mp4")
		require.Equal(t, "x.mp4", got, "url base = %q, want x.mp4", got)
	}

	{
		got := StoredPathBase("/var/lib/videos/h/a.mp4")
		require.Equal(t, "a.mp4", got, "posix absolute base = %q, want a.mp4", got)
	}

	{
		got := StoredPathBase("  ")
		require.Empty(t, got, "blank base = %q, want empty", got)
	}

	// 带盘符的绝对路径：保留原行为（返回 "."）。
	{
		got := StoredPathBase(`D:\tmp\a.mp4`)
		require.Equal(t, ".", got, "drive-letter path base = %q, want \".\" (legacy behavior)", got)
	}
}
