package storage

import (
	"path/filepath"
	"strings"
	"testing"

	"f_ccg_video_back_ddd/internal/infrastructure/config/site"
)

// testStorageConfig 构造一份本地 + MinIO 双驱动配置（不连接真实服务）。
func testStorageConfig() *site.Storage {
	return &site.Storage{
		Local: &site.Local{Path: "uploads"},
		MinIO: &site.MinIO{Endpoint: "localhost:9002", Bucket: "videos"},
	}
}

func TestMinIOPublicPath(t *testing.T) {
	t.Parallel()

	m := &MinIOStorage{cfg: testStorageConfig(), bucket: "videos"}

	cases := []struct{ name, key, want string }{
		{"plain key", "user_files/images/x.jpg", "/user_files/images/x.jpg"},
		{"already slashed", "/user_files/images/x.jpg", "/user_files/images/x.jpg"},
		{"video object", "user_files/videos/h/_source_h.mp4", "/user_files/videos/h/_source_h.mp4"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := m.publicPath(tc.key); got != tc.want {
				t.Fatalf("publicPath(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestMinIOResolveSource(t *testing.T) {
	t.Parallel()

	m := &MinIOStorage{cfg: testStorageConfig(), bucket: "videos"}

	// 根相对路径 → 后端直连绝对 URL（host 不出现在对外响应里）
	got, err := m.ResolveSource(t.Context(), "/user_files/images/x.jpg")
	if err != nil {
		t.Fatalf("ResolveSource: %v", err)
	}

	if want := "http://localhost:9002/videos/user_files/images/x.jpg"; got != want {
		t.Fatalf("ResolveSource = %q, want %q", got, want)
	}

	// 历史绝对 URL（存量数据）→ 原样返回
	legacy := "http://minio:9002/videos/user_files/images/x.jpg"
	if got, err = m.ResolveSource(t.Context(), legacy); err != nil || got != legacy {
		t.Fatalf("ResolveSource(legacy) = %q, %v; want %q", got, err, legacy)
	}

	// 空路径 → 空串，不报错
	if got, err = m.ResolveSource(t.Context(), "  "); err != nil || got != "" {
		t.Fatalf("ResolveSource(empty) = %q, %v; want empty", got, err)
	}
}

func TestMinIOObjectKeyFromPathRelativeAndLegacy(t *testing.T) {
	t.Parallel()

	m := &MinIOStorage{cfg: testStorageConfig(), bucket: "videos"}

	// 新契约：根相对路径（无 bucket）
	if key, err := m.objectKeyFromPath("/user_files/images/x.jpg"); err != nil || key != "user_files/images/x.jpg" {
		t.Fatalf("objectKeyFromPath(relative) = %q, %v", key, err)
	}

	// 旧数据：完整 URL（含 bucket），仍可提取 key
	if key, err := m.objectKeyFromPath("http://host/videos/user_files/images/x.jpg"); err != nil || key != "user_files/images/x.jpg" {
		t.Fatalf("objectKeyFromPath(legacy url) = %q, %v", key, err)
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

			if got := l.publicPath(tc.rel); got != tc.want {
				t.Fatalf("publicPath(%q) = %q, want %q", tc.rel, got, tc.want)
			}
		})
	}
}

func TestLocalResolveSource(t *testing.T) {
	t.Parallel()

	l := &LocalStorage{cfg: testStorageConfig()}
	base, err := filepath.Abs("uploads")
	if err != nil {
		t.Fatalf("abs base: %v", err)
	}

	// 根相对路径（含 /uploads 根前缀）→ base 内绝对路径
	got, err := l.ResolveSource(t.Context(), "/uploads/images/x.jpg")
	if err != nil {
		t.Fatalf("ResolveSource: %v", err)
	}

	want := filepath.Join(base, "images", "x.jpg")
	if got != want {
		t.Fatalf("ResolveSource = %q, want %q", got, want)
	}

	// 根前缀必须被剥离，不能出现 uploads/uploads 重复拼接
	if strings.Contains(got, "uploads"+string(filepath.Separator)+"uploads") {
		t.Fatalf("ResolveSource double-prefixed base: %q", got)
	}

	// 绝对路径 → 原样返回
	if got, err = l.ResolveSource(t.Context(), want); err != nil || got != want {
		t.Fatalf("ResolveSource(abs) = %q, %v", got, err)
	}
}
