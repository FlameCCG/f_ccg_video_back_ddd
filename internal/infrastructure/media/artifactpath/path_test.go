package artifactpath

import (
	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"strings"
	"testing"
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
			relative: "videos/hash/x_720p.mp4",
			want:     true,
		},
		{
			name:     "plain relative path",
			stored:   "videos/hash/x_720p.mp4",
			relative: "videos/hash/x_720p.mp4",
			want:     true,
		},
		{
			name:     "different object",
			stored:   "http://minio:9000/bucket/videos/other/x_720p.mp4",
			relative: "videos/hash/x_720p.mp4",
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
			relative: "videos/hash/x_720p.mp4",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := (Policy{}).StoredPathMatchesRelative(tt.stored, tt.relative); got != tt.want {
				t.Fatalf("(Policy{}).StoredPathMatchesRelative(%q, %q) = %v, want %v",
					tt.stored, tt.relative, got, tt.want)
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

			if got := (Policy{}).ShouldDeleteOriginalSource(tt.initial, tt.current); got != tt.want {
				t.Fatalf("ShouldDeleteOriginalSource(%q, %q) = %v, want %v",
					tt.initial, tt.current, got, tt.want)
			}
		})
	}
}

// TestStoredPathBase 覆盖相对反斜杠路径与完整 URL。
//
// `D:\tmp\a.mp4` 等带盘符的绝对路径会被 url.Parse 识别为 scheme "d"，
// 取到空 u.Path 后返回 "."。转码产物应使用相对对象 key 或 http(s) URL，不使用盘符路径。
func TestStoredPathBase(t *testing.T) {
	t.Parallel()

	if got := (Policy{}).pathBase(`videos\h\x.mp4`); got != "x.mp4" {
		t.Fatalf("relative backslash base = %q, want x.mp4", got)
	}

	if got := (Policy{}).pathBase("http://minio:9000/bucket/videos/h/x.mp4"); got != "x.mp4" {
		t.Fatalf("url base = %q, want x.mp4", got)
	}

	if got := (Policy{}).pathBase("/var/lib/videos/h/a.mp4"); got != "a.mp4" {
		t.Fatalf("posix absolute base = %q, want a.mp4", got)
	}

	if got := (Policy{}).pathBase("  "); got != "" {
		t.Fatalf("blank base = %q, want empty", got)
	}

	// 带盘符的绝对路径返回 "."。
	if got := (Policy{}).pathBase(`D:\tmp\a.mp4`); got != "." {
		t.Fatalf("drive-letter path base = %q, want \".\" (legacy behavior)", got)
	}
}

func TestBuildSourceNormalizeTargetsUsesIsolatedObject(t *testing.T) {
	t.Parallel()

	task := mediadomain.TranscodeTask{
		VideoID:    9,
		FileHash:   "src",
		OutputPath: "video/9",
		SourcePath: "http://localhost/videos/video/9/_source_src.mp4",
	}

	first := (mediadomain.OutputPlanner{}).BuildSourceNormalizeTargets(task, 1080, 1080, enum.Resolution1080P, false)

	second := (mediadomain.OutputPlanner{}).BuildSourceNormalizeTargets(task, 1080, 1080, enum.Resolution1080P, false)
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("want one source target per build, got %d and %d", len(first), len(second))
	}

	if first[0].FileName == "_source_src.mp4" {
		t.Fatalf("normalized target reused source object name %q", first[0].FileName)
	}

	if !strings.HasPrefix(first[0].FileName, "_normalized_src") {
		t.Fatalf("normalized target name = %q, want isolated prefix", first[0].FileName)
	}
	// 断点续传要求确定性命名：重试按名字匹配已提交/已入库的规范化产物并原地覆盖。
	if first[0].RelativePath != second[0].RelativePath {
		t.Fatalf("normalized target path must be deterministic across attempts: %q vs %q",
			first[0].RelativePath, second[0].RelativePath)
	}

	if err := (Policy{}).ValidateSourceTargetIsolation(task.SourcePath, first); err != nil {
		t.Fatalf("isolated target rejected: %v", err)
	}
}

func TestValidateSourceTargetIsolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  mediadomain.TranscodeTarget
		wantErr bool
	}{
		{
			name: "reject source target with same object name",
			target: mediadomain.TranscodeTarget{
				IsSource:     true,
				RelativePath: "user_files/hash/_source_hash.mp4",
			},
			wantErr: true,
		},
		{
			name: "allow isolated normalized source target",
			target: mediadomain.TranscodeTarget{
				IsSource:     true,
				RelativePath: "user_files/hash/_normalized_hash_attempt.mp4",
			},
		},
		{
			name: "derived target is not the source replacement",
			target: mediadomain.TranscodeTarget{
				RelativePath: "user_files/hash/_source_hash.mp4",
			},
		},
	}

	const source = "http://localhost:9002/videos/user_files/hash/_source_hash.mp4"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (Policy{}).ValidateSourceTargetIsolation(source, []mediadomain.TranscodeTarget{tt.target})
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateSourceTargetIsolation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
