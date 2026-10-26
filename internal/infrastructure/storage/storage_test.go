package storage

import (
	"f_ccg_video_back_ddd/internal/config/site"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// testStorageConfig 构造一份本地 + MinIO 双驱动配置（不连接真实服务）。
func testStorageConfig() *site.Storage {
	return &site.Storage{
		Local: &site.Local{Path: "uploads"},
		MinIO: &site.MinIO{Endpoint: "localhost:9002", Bucket: testVideos},
	}
}

func TestMinIOPublicPath(t *testing.T) {
	t.Parallel()

	m := &MinIOStorage{cfg: testStorageConfig(), bucket: testVideos}

	cases := []struct{ name, key, want string }{
		{"plain key", "user_files/images/x.jpg", "/user_files/images/x.jpg"},
		{"already slashed", "/user_files/images/x.jpg", "/user_files/images/x.jpg"},
		{"video object", "user_files/videos/h/_source_h.mp4", "/user_files/videos/h/_source_h.mp4"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			{
				got := m.publicPath(tc.key)
				require.Equal(t, tc.want, got, "publicPath(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestMinIOResolveSource(t *testing.T) {
	t.Parallel()

	m := &MinIOStorage{cfg: testStorageConfig(), bucket: testVideos}

	// 根相对路径 → 后端直连绝对 URL（host 不出现在对外响应里）
	got, err := m.ResolveSource(t.Context(), "/user_files/images/x.jpg")
	require.NoError(t, err, "ResolveSource: %v", err)

	{
		want := "http://localhost:9002/videos/user_files/images/x.jpg"
		require.Equal(t, want, got, "ResolveSource = %q, want %q", got, want)
	}

	// 历史绝对 URL（存量数据）→ 原样返回
	legacy := "http://minio:9002/videos/user_files/images/x.jpg"
	{
		got, err = m.ResolveSource(t.Context(), legacy)
		require.NoError(t, err, "ResolveSource(legacy) = %q, %v; want %q", got, err, legacy)
		require.Equal(t, got, legacy, "ResolveSource(legacy) = %q, %v; want %q", got, err, legacy)
	}

	// 空路径 → 空串，不报错
	{
		got, err = m.ResolveSource(t.Context(), "  ")
		require.NoError(t, err, "ResolveSource(empty) = %q, %v; want empty", got, err)
		require.Empty(t, got, "ResolveSource(empty) = %q, %v; want empty", got, err)
	}
}

func TestMinIOObjectKeyFromPathRelativeAndLegacy(t *testing.T) {
	t.Parallel()

	m := &MinIOStorage{cfg: testStorageConfig(), bucket: testVideos}

	// 新契约：根相对路径（无 bucket）
	{
		key, err := m.objectKeyFromPath("/user_files/images/x.jpg")
		require.NoError(t, err, "objectKeyFromPath(relative) = %q, %v", key, err)
		require.Equal(t, "user_files/images/x.jpg", key, "objectKeyFromPath(relative) = %q, %v", key, err)
	}

	// 旧数据：完整 URL（含 bucket），仍可提取 key
	{
		key, err := m.objectKeyFromPath("http://host/videos/user_files/images/x.jpg")
		require.NoError(t, err, "objectKeyFromPath(legacy url) = %q, %v", key, err)
		require.Equal(t, "user_files/images/x.jpg", key, "objectKeyFromPath(legacy url) = %q, %v", key, err)
	}
}

func TestLocalPublicPath(t *testing.T) {
	t.Parallel()

	l := &LocalStorage{cfg: testStorageConfig()}

	cases := []struct{ name, rel, want string }{
		{"image", "images/x.jpg", "/uploads/images/x.jpg"},
		{"report", "reports/video/x.jpg", "/uploads/reports/video/x.jpg"},
		{"already slashed", "/images/x.jpg", "/uploads/images/x.jpg"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			{
				got := l.publicPath(tc.rel)
				require.Equal(t, tc.want, got, "publicPath(%q) = %q, want %q", tc.rel, got, tc.want)
			}
		})
	}
}

func TestLocalResolveSource(t *testing.T) {
	t.Parallel()

	l := &LocalStorage{cfg: testStorageConfig()}

	base, err := filepath.Abs("uploads")
	require.NoError(t, err, "abs base: %v", err)

	// 根相对路径（含 /uploads 根前缀）→ base 内绝对路径
	got, err := l.ResolveSource(t.Context(), "/uploads/images/x.jpg")
	require.NoError(t, err, "ResolveSource: %v", err)

	want := filepath.Join(base, "images", "x.jpg")
	require.Equal(t, want, got, "ResolveSource = %q, want %q", got, want)

	// 根前缀必须被剥离，不能出现 uploads/uploads 重复拼接
	require.NotContains(t, got, "uploads"+string(filepath.Separator)+"uploads", "ResolveSource double-prefixed base: %q", got)

	// 绝对路径 → 原样返回
	{
		got, err = l.ResolveSource(t.Context(), want)
		require.NoError(t, err, "ResolveSource(abs) = %q, %v", got, err)
		require.Equal(t, want, got, "ResolveSource(abs) = %q, %v", got, err)
	}
}
