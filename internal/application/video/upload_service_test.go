package video_test

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"go.uber.org/zap"
)

// uploadIDPattern 与 infrastructure/storage.ValidateUploadID 的规则等价：
// 允许十六进制哈希、带后缀的自定义标识与历史标识，拒绝空串与路径分隔符。
var uploadIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.\-]+$`)

// stubDriver 存储驱动 stub（等价原 upload_service_test.go 的 stubDriver）。
type stubDriver struct {
	completedFilePath    string
	completed            bool
	uploadedChunks       []string
	completedErr         error
	chunkErr             error
	mergePath            string
	waitCompletedTimeout bool
	waitChunkTimeout     bool
	listChunkCalls       int
}

func (s *stubDriver) SaveChunk(context.Context, string, string, int64, io.Reader) error {
	return nil
}

func (s *stubDriver) ListUploadedChunks(ctx context.Context, _ string) ([]string, error) {
	s.listChunkCalls++

	if s.waitChunkTimeout {
		<-ctx.Done()

		return nil, ctx.Err()
	}

	if s.chunkErr != nil {
		return nil, s.chunkErr
	}

	return append([]string(nil), s.uploadedChunks...), nil
}

func (s *stubDriver) GetCompletedFile(ctx context.Context, _ string) (string, bool, error) {
	if s.waitCompletedTimeout {
		<-ctx.Done()

		return "", false, ctx.Err()
	}

	if s.completedErr != nil {
		return "", false, s.completedErr
	}

	return s.completedFilePath, s.completed, nil
}

func (s *stubDriver) MergeChunks(context.Context, string, string, int) (string, error) {
	return s.mergePath, nil
}

func (s *stubDriver) SaveFile(context.Context, string, int64, io.Reader, string) (string, error) {
	return "", nil
}

func (s *stubDriver) DeleteStoredFile(context.Context, string) error {
	return nil
}

func (s *stubDriver) DeleteByPrefix(context.Context, string) (int64, error) {
	return 0, nil
}

// stubResolver 存储解析端口 stub：固定返回同一个 driver。
type stubResolver struct {
	driver *stubDriver
	err    error
}

func (r *stubResolver) Resolve() (app.StorageDriver, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.driver, nil
}

func (r *stubResolver) ResolveByType(enum.VideoStorageType) (app.StorageDriver, error) {
	return r.Resolve()
}

func (r *stubResolver) ValidateUploadID(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return errors.New("上传标识不能为空")
	}

	if !uploadIDPattern.MatchString(trimmed) {
		return errors.New("上传标识格式无效")
	}

	return nil
}

func (r *stubResolver) ValidateChunkIndex(index string) error {
	n, err := strconv.Atoi(strings.TrimSpace(index))
	if err != nil || n < 0 {
		return errors.New("分片索引格式无效")
	}

	return nil
}

// newUploadService 用 stub driver 装配真实 UploadService（测试不走 wire）。
func newUploadService(driver *stubDriver) *app.UploadService {
	return app.NewUploadService(&stubResolver{driver: driver}, infraconfig.NewProvider(nil), zap.NewNop())
}

// TestValidateUploadID 覆盖原 TestValidateFileHash：上传标识的接受/拒绝集合。
func TestValidateUploadID(t *testing.T) {
	svc := newUploadService(&stubDriver{})

	tests := []struct {
		name     string
		fileHash string
		wantErr  bool
	}{
		{
			name:     "accept sha256",
			fileHash: "69af04e1bb64adb459e3e0038bb7b530e3467117ccac259cac2bbf770ecb7446",
		},
		{
			name:     "accept md5",
			fileHash: "6f5902ac237024bdd0c176cb93063dc4",
		},
		{
			name:     "accept custom upload identifier",
			fileHash: "69af04e1bb64adb459e3e0038bb7b530e3467117ccac259cac2bbf770ecb7446-37402917-1736784301787",
		},
		{
			name:     "accept arbitrary legacy identifier",
			fileHash: "upload-task-1736784301787",
		},
		{
			name:     "reject empty identifier",
			fileHash: "   ",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateUploadID(strings.TrimSpace(tt.fileHash))
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tt.fileHash)
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tt.fileHash, err)
			}
		})
	}
}

// TestGetUploadStatus 覆盖分片上传状态查询的四条路径 + 硬错误可见性。
func TestGetUploadStatus(t *testing.T) {
	t.Run("returns completed file for instant upload", func(t *testing.T) {
		svc := newUploadService(&stubDriver{
			completed:         true,
			completedFilePath: "/uploads/videos/hash/_source_hash.mp4",
		})

		status, err := svc.GetUploadStatus(t.Context(), "hash")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !status.Completed {
			t.Fatal("expected completed status")
		}

		if status.FileHash != "hash" {
			t.Fatalf("expected file hash to be echoed back, got %q", status.FileHash)
		}

		if status.FilePath == "" {
			t.Fatal("expected file path for completed upload")
		}

		if len(status.UploadedChunks) != 0 {
			t.Fatalf("expected no chunk list for completed upload, got %v", status.UploadedChunks)
		}
	})

	t.Run("returns uploaded chunks for resumable upload", func(t *testing.T) {
		svc := newUploadService(&stubDriver{uploadedChunks: []string{"0", "2"}})

		status, err := svc.GetUploadStatus(t.Context(), "hash")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if status.Completed {
			t.Fatal("expected incomplete status")
		}

		if status.FileHash != "hash" {
			t.Fatalf("expected file hash to be echoed back, got %q", status.FileHash)
		}

		if status.FilePath != "" {
			t.Fatalf("expected empty file path, got %q", status.FilePath)
		}

		if len(status.UploadedChunks) != 2 {
			t.Fatalf("expected uploaded chunks, got %v", status.UploadedChunks)
		}
	})

	t.Run("falls back to chunk list when completed probe times out", func(t *testing.T) {
		svc := newUploadService(&stubDriver{
			waitCompletedTimeout: true,
			uploadedChunks:       []string{"1", "0"},
		})

		status, err := svc.GetUploadStatus(t.Context(), "hash")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if status.Completed {
			t.Fatal("expected incomplete status")
		}

		if len(status.UploadedChunks) != 2 ||
			status.UploadedChunks[0] != "0" ||
			status.UploadedChunks[1] != "1" {
			t.Fatalf("expected sorted uploaded chunks, got %v", status.UploadedChunks)
		}
	})

	t.Run("degrades to empty status on transient storage errors", func(t *testing.T) {
		svc := newUploadService(&stubDriver{
			waitCompletedTimeout: true,
			waitChunkTimeout:     true,
		})

		status, err := svc.GetUploadStatus(t.Context(), "hash")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if status.Completed {
			t.Fatal("expected incomplete status")
		}

		if len(status.UploadedChunks) != 0 {
			t.Fatalf("expected empty chunk list, got %v", status.UploadedChunks)
		}
	})

	t.Run("keeps hard errors visible when probes are not transient", func(t *testing.T) {
		svc := newUploadService(&stubDriver{
			completedErr: errors.New("minio auth failed"),
			chunkErr:     errors.New("minio auth failed"),
		})

		if _, err := svc.GetUploadStatus(t.Context(), "hash"); err == nil {
			t.Fatal("expected hard error to be returned")
		}
	})
}

// TestCompleteUploadWarmsCompletedCache 合并成功后 status 应报告已完成且路径一致。
func TestCompleteUploadWarmsCompletedCache(t *testing.T) {
	driver := &stubDriver{
		uploadedChunks:    []string{"0"},
		mergePath:         "/uploads/videos/hash/_source_hash.mp4",
		completed:         true,
		completedFilePath: "/uploads/videos/hash/_source_hash.mp4",
	}
	svc := newUploadService(driver)

	finalPath, err := svc.CompleteUpload(t.Context(), "hash", "demo.mp4", 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if finalPath == "" {
		t.Fatal("expected merged file path")
	}

	status, err := svc.GetUploadStatus(t.Context(), "hash")
	if err != nil {
		t.Fatalf("expected status ok, got %v", err)
	}

	if !status.Completed {
		t.Fatal("expected completed status from storage")
	}

	if status.FilePath != finalPath {
		t.Fatalf("expected file path %q, got %q", finalPath, status.FilePath)
	}
}

// TestGetUploadStatusInvalidatesStaleCompletedCache 对象已消失时不得继续报告秒传成功。
func TestGetUploadStatusInvalidatesStaleCompletedCache(t *testing.T) {
	// 先用一次成功合并把 "已完成" 写进进程内缓存。
	warm := &stubDriver{
		uploadedChunks:    []string{"0"},
		mergePath:         "/gone.mp4",
		completed:         false,
		completedFilePath: "",
	}
	svc := newUploadService(warm)

	if _, err := svc.CompleteUpload(t.Context(), "hash", "demo.mp4", 1); err != nil {
		t.Fatalf("warm complete upload failed: %v", err)
	}

	// 对象随后被硬删：GetUploadStatus 必须以存储为准。
	status, err := svc.GetUploadStatus(t.Context(), "hash")
	if err != nil {
		t.Fatalf("expected status ok, got %v", err)
	}

	if status.Completed {
		t.Fatal("stale cache must not report completed when object is gone")
	}
}

// TestExtractSourceUploadHash 覆盖从源对象路径提取 fileHash（领域纯函数）。
func TestExtractSourceUploadHash(t *testing.T) {
	svc := newUploadService(&stubDriver{})
	validate := svc.ValidateUploadID

	hash := "69af04e1bb64adb459e3e0038bb7b530e3467117ccac259cac2bbf770ecb7446"

	got := domain.ExtractSourceUploadHash(
		"http://localhost:9000/bucket/user_files/"+hash+"/_source_"+hash+".mp4",
		validate,
	)
	if got != hash {
		t.Fatalf("got %q want %q", got, hash)
	}

	if domain.ExtractSourceUploadHash("videos/cover-only/preview.jpg", validate) != "" {
		t.Fatal("expected empty for non-source path")
	}
}

// TestCompleteUploadReusesExistingWithoutChunks 最终对象已存在时直接复用，不校验分片完整性。
func TestCompleteUploadReusesExistingWithoutChunks(t *testing.T) {
	driver := &stubDriver{
		completed:         true,
		completedFilePath: "/uploads/videos/hash/_source_hash.mp4",
		// no chunks — previously would fail ensureChunkIntegrity
	}
	svc := newUploadService(driver)

	path, err := svc.CompleteUpload(t.Context(), "hash", "demo.mp4", 4)
	if err != nil {
		t.Fatalf("expected reuse, got %v", err)
	}

	if path != driver.completedFilePath {
		t.Fatalf("path = %q, want %q", path, driver.completedFilePath)
	}
}
