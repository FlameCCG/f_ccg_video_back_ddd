package video

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/logctx"
	"f_ccg_video_back_ddd/pkg/uploadscope"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

// uploadStatusProbeTimeout 上传状态探测的单次超时。
const uploadStatusProbeTimeout = 4 * time.Second

// ErrUploadStorageUnavailable 存储驱动不可用（原 NewUploadService 失败时的响应文案）。
var ErrUploadStorageUnavailable = errors.New("上传配置错误")

// UploadService 分片上传用例服务（原 service/video_service/upload_service.go）。
//
// 合并锁由服务实例持有；完成状态直接探测存储，避免源文件删除后误判秒传。
type UploadService struct {
	storage     StorageResolver
	cfgProvider shared.ConfigReader
	quota       *UploadQuotaService
	guard       UploadHashGuard
	mergeLocks  *mergeLockManager
	logger      *zap.Logger
}

// NewUploadService 构造分片上传用例服务。
func NewUploadService(
	storage StorageResolver,
	cfgProvider shared.ConfigReader,
	quota *UploadQuotaService,
	guard UploadHashGuard,
	logger *zap.Logger,
) *UploadService {
	return &UploadService{
		storage:     storage,
		cfgProvider: cfgProvider,
		quota:       quota,
		guard:       guard,
		mergeLocks:  newMergeLockManager(),
		logger:      logger,
	}
}

// UploadChunkForUser 在首片实际写入前占用当天名额；失败或放弃均不退还。
func (s *UploadService) UploadChunkForUser(ctx context.Context, userID uint, fileHash, index string, size int64, reader io.Reader) error {
	ctx = uploadscope.WithOwner(ctx, userID)
	fileHash = trimSpace(fileHash)
	index = trimSpace(index)

	if err := s.validateFileHash(fileHash); err != nil {
		return err
	}

	if err := s.storage.ValidateChunkIndex(index); err != nil {
		return err
	}

	if err := s.claimQuota(ctx, userID, fileHash); err != nil {
		return err
	}

	return s.withHashes(ctx, []string{fileHash, uploadscope.ChunkNamespace(ctx, fileHash)}, func(guardCtx context.Context) error {
		return s.UploadChunk(guardCtx, fileHash, index, size, reader)
	})
}

// GetUploadStatusForUser 仅在确认完整对象存在后为秒传占额，才返回 completed=true。
func (s *UploadService) GetUploadStatusForUser(ctx context.Context, userID uint, fileHash string) (*UploadStatus, error) {
	ctx = uploadscope.WithOwner(ctx, userID)

	fileHash = trimSpace(fileHash)
	if err := s.validateFileHash(fileHash); err != nil {
		return nil, err
	}

	var status *UploadStatus

	err := s.withHashes(ctx, []string{fileHash, uploadscope.ChunkNamespace(ctx, fileHash)}, func(guardCtx context.Context) error {
		var err error

		status, err = s.GetUploadStatus(guardCtx, fileHash)
		if err != nil || !status.Completed {
			return err
		}

		return s.claimQuota(guardCtx, userID, fileHash)
	})

	return status, err
}

// CompleteUploadForUser 在复用已完成对象或合并分片前做服务端额度兜底。
func (s *UploadService) CompleteUploadForUser(ctx context.Context, userID uint, fileHash, fileName string, totalChunks int) (string, error) {
	ctx = uploadscope.WithOwner(ctx, userID)

	fileHash = trimSpace(fileHash)
	if err := s.validateFileHash(fileHash); err != nil {
		return "", err
	}

	if err := s.claimQuota(ctx, userID, fileHash); err != nil {
		return "", err
	}

	var path string

	err := s.withHashes(ctx, []string{fileHash, uploadscope.ChunkNamespace(ctx, fileHash)}, func(guardCtx context.Context) error {
		var err error

		path, err = s.CompleteUpload(guardCtx, fileHash, fileName, totalChunks)

		return err
	})

	return path, err
}

// WithPublishUploads 将多分P的对象校验、额度检查和 MySQL 投稿写入置于同一组哈希锁内。
func (s *UploadService) WithPublishUploads(ctx context.Context, userID uint, hashes []string, validate, persist func(context.Context) error) error {
	normalized := make([]string, len(hashes))
	for i, hash := range hashes {
		hash = trimSpace(hash)
		if err := s.validateFileHash(hash); err != nil {
			return err
		}

		normalized[i] = hash
	}

	return s.withHashes(ctx, normalized, func(guardCtx context.Context) error {
		if err := validate(guardCtx); err != nil {
			return err
		}

		for _, hash := range uniqueUploadHashes(normalized) {
			if err := s.claimQuota(guardCtx, userID, hash); err != nil {
				return err
			}
		}

		return persist(guardCtx)
	})
}

func (s *UploadService) claimQuota(ctx context.Context, userID uint, fileHash string) error {
	if s.quota == nil { // 仅供不装配 Redis 的单元测试使用；正式 Wire 图始终注入 quota。
		return nil
	}

	return s.quota.Claim(ctx, userID, fileHash)
}

func (s *UploadService) withHashes(ctx context.Context, hashes []string, fn func(context.Context) error) error {
	if s.guard == nil { // 仅供不装配 Redis 的单元测试使用。
		return fn(ctx)
	}

	return s.guard.Run(ctx, hashes, fn)
}

// UploadChunk 保存一个分片。
func (s *UploadService) UploadChunk(
	ctx context.Context,
	fileHash, index string,
	size int64,
	reader io.Reader,
) error {
	fileHash = trimSpace(fileHash)
	index = trimSpace(index)

	driver, err := s.resolve()
	if err != nil {
		return err
	}

	if err := s.validateFileHash(fileHash); err != nil {
		return err
	}

	if err := s.storage.ValidateChunkIndex(index); err != nil {
		return err
	}

	if err := s.validateIncomingChunkSize(ctx, driver, fileHash, index, size); err != nil {
		return err
	}

	return driver.SaveChunk(ctx, fileHash, index, size, reader)
}

// GetUploadedChunks 读取已上传的分片索引列表。
func (s *UploadService) GetUploadedChunks(ctx context.Context, fileHash string) ([]string, error) {
	fileHash = trimSpace(fileHash)

	driver, err := s.resolve()
	if err != nil {
		return nil, err
	}

	if err := s.validateFileHash(fileHash); err != nil {
		return nil, err
	}

	return driver.ListUploadedChunks(ctx, fileHash)
}

// GetUploadStatus 查询分片上传状态。
//
// completed 必须以对象存储为准；内存缓存只做加速，对象被硬删后不得继续秒传。
func (s *UploadService) GetUploadStatus(ctx context.Context, fileHash string) (*UploadStatus, error) {
	fileHash = trimSpace(fileHash)

	driver, err := s.resolve()
	if err != nil {
		return nil, err
	}

	if err := s.validateFileHash(fileHash); err != nil {
		return nil, err
	}

	statusCtx, cancelStatus := context.WithCancel(orTODO(ctx))
	defer cancelStatus()

	completedCh := make(chan completedFileProbeResult, 1)
	chunksCh := make(chan uploadedChunksProbeResult, 1)

	go func() {
		probeCtx, cancel := ctxutil.WithTimeout(statusCtx, uploadStatusProbeTimeout)
		defer cancel()

		filePath, completed, probeErr := driver.GetCompletedFile(probeCtx, fileHash)
		completedCh <- completedFileProbeResult{
			filePath:  filePath,
			completed: completed,
			err:       probeErr,
		}
	}()

	go func() {
		probeCtx, cancel := ctxutil.WithTimeout(statusCtx, uploadStatusProbeTimeout)
		defer cancel()

		chunks, probeErr := driver.ListUploadedChunks(probeCtx, fileHash)
		if probeErr == nil {
			sort.Strings(chunks)
		}

		chunksCh <- uploadedChunksProbeResult{
			chunks: chunks,
			err:    probeErr,
		}
	}()

	completedRes := <-completedCh
	if completedRes.err == nil && completedRes.completed && trimSpace(completedRes.filePath) != "" {
		cancelStatus()

		return &UploadStatus{
			FileHash:       fileHash,
			Completed:      true,
			FilePath:       completedRes.filePath,
			UploadedChunks: []string{},
		}, nil
	}

	chunksRes := <-chunksCh
	if chunksRes.err == nil {
		return &UploadStatus{
			FileHash:       fileHash,
			Completed:      false,
			FilePath:       "",
			UploadedChunks: chunksRes.chunks,
		}, nil
	}

	if isTransientUploadStatusError(completedRes.err) || isTransientUploadStatusError(chunksRes.err) {
		logctx.With(ctx, s.logger).Warn("upload status degraded due to transient storage error", zap.String("event", "video.upload_service.get_upload_status.degraded"), zap.String("operation", "video.upload_service.get_upload_status"),
			zap.String("file_hash", fileHash),
			zap.NamedError("completed_error", completedRes.err),
			zap.NamedError("chunks_error", chunksRes.err))

		return &UploadStatus{
			FileHash:       fileHash,
			Completed:      false,
			FilePath:       "",
			UploadedChunks: []string{},
		}, nil
	}

	switch {
	case completedRes.err == nil:
		return nil, chunksRes.err
	case chunksRes.err == nil:
		return nil, completedRes.err
	default:
		return nil, errors.Join(completedRes.err, chunksRes.err)
	}
}

// CompleteUpload 合并分片并返回最终文件路径。
func (s *UploadService) CompleteUpload(
	ctx context.Context,
	fileHash, fileName string,
	totalChunks int,
) (string, error) {
	driver, err := s.resolve()
	if err != nil {
		return "", err
	}

	fileHash = trimSpace(fileHash)

	fileName = trimSpace(fileName)

	if err := s.validateFileHash(fileHash); err != nil {
		return "", err
	}

	if err := domain.ValidateCompleteUpload(fileName, totalChunks); err != nil {
		return "", err
	}

	unlock := s.mergeLocks.lock(fileHash)
	defer unlock()

	// 最终对象已在存储中：直接复用（分片可能已在上次 complete 后清理）
	if existing, ok, probeErr := driver.GetCompletedFile(ctx, fileHash); probeErr == nil && ok && trimSpace(existing) != "" {
		logctx.With(ctx, s.logger).Info("complete upload reused existing object", zap.String("event", "video.upload_service.complete_upload.diagnostic"), zap.String("operation", "video.upload_service.complete_upload"),
			zap.String("file_hash", fileHash),
			zap.String("file_path", existing))

		return existing, nil
	}

	if err := s.ensureChunkIntegrity(ctx, driver, fileHash, totalChunks); err != nil {
		return "", err
	}

	if err := s.validateIncomingChunkSize(ctx, driver, fileHash, "", 0); err != nil {
		return "", err
	}

	start := time.Now()

	finalPath, err := driver.MergeChunks(ctx, fileHash, fileName, totalChunks)
	if err != nil {
		return "", err
	}

	logctx.With(ctx, s.logger).Info("complete upload succeeded", zap.String("event", "video.upload_service.complete_upload.diagnostic"), zap.String("operation", "video.upload_service.complete_upload"),
		zap.String("file_hash", fileHash),
		zap.String("file_name", fileName),
		zap.Int("total_chunks", totalChunks),
		zap.Duration("duration_ms", time.Since(start)))

	return finalPath, nil
}

// InvalidateUploadCompletedCache 保留删除用例的兼容端口；完成状态已不再缓存。
func (s *UploadService) InvalidateUploadCompletedCache(_ string) {
	// Completion is always probed directly from storage.
}

// InvalidateUploadCompletedCacheByStoredPath 从对象路径中尽力提取 fileHash 后失效缓存。
func (s *UploadService) InvalidateUploadCompletedCacheByStoredPath(storedPath string) {
	hash := domain.ExtractSourceUploadHash(storedPath, s.storage.ValidateUploadID)
	if hash == "" {
		return
	}

	s.InvalidateUploadCompletedCache(hash)
}

// ResolveCompletedUpload 校验并返回已完成上传对象的路径（投稿流程使用）。
func (s *UploadService) ResolveCompletedUpload(ctx context.Context, fileHash string) (string, error) {
	driver, err := s.resolve()
	if err != nil {
		return "", errStorageUnavailable
	}

	if err := s.storage.ValidateUploadID(fileHash); err != nil {
		return "", err
	}

	probeCtx, cancel := ctxutil.WithTimeout(ctx, ctxutil.RequestSlowTimeout)
	defer cancel()

	filePath, completed, err := driver.GetCompletedFile(probeCtx, trimSpace(fileHash))
	if err != nil {
		return "", apperr.Wrap("video.upload.check", errCheckUploadFailed, err)
	}

	if !completed || trimSpace(filePath) == "" {
		logctx.With(ctx, s.logger).Warn("publish blocked: completed upload object missing", zap.String("event", "video.upload_service.resolve_completed_upload.degraded"), zap.String("operation", "video.upload_service.resolve_completed_upload"),
			zap.String("file_hash", trimSpace(fileHash)))

		return "", errUploadIncomplete
	}

	return filePath, nil
}

// SaveImage 保存一个图片对象并返回可访问 URL（封面 / 举报图片共用）。
func (s *UploadService) SaveImage(
	ctx context.Context,
	relativePath string,
	size int64,
	reader io.Reader,
	contentType string,
) (string, error) {
	driver, err := s.resolve()
	if err != nil {
		return "", err
	}

	return driver.SaveFile(ctx, relativePath, size, reader, contentType)
}

// ValidateUploadID 暴露上传标识校验，供 handler 在读取 multipart 前做前置校验。
func (s *UploadService) ValidateUploadID(id string) error {
	return s.storage.ValidateUploadID(id)
}

// EnsureStorageReady 校验存储驱动配置可用。
//
// 对应原 saveReportImages 中「ResolveDriver 失败 → 存储配置错误」的前置分支：
// handler 需要在遍历上传文件之前先失败，避免部分文件已落盘后才报错。
func (s *UploadService) EnsureStorageReady() error {
	_, err := s.resolve()

	return err
}

func (s *UploadService) resolve() (StorageDriver, error) {
	driver, err := s.storage.Resolve()
	if err != nil {
		return nil, apperr.Wrap("application/video/upload_service", ErrUploadStorageUnavailable, err)
	}

	return driver, nil
}

func (s *UploadService) validateFileHash(fileHash string) error {
	return s.storage.ValidateUploadID(trimSpace(fileHash))
}

const mebibyte int64 = 1024 * 1024

// uploadSizeLimits 读取单分片和整文件上限，单位 MB。
// maxChunkSize 优先；没配时用 chunkSize 卡住单分片。两者都没配则不限制单分片。
func (s *UploadService) uploadSizeLimits() (chunkMB, fileMB int64) {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil || cfg.Site.Storage == nil {
		return 0, 0
	}

	storage := cfg.Site.Storage

	chunkMB = storage.MaxChunkSize
	if chunkMB <= 0 {
		chunkMB = storage.ChunkSize
	}

	return chunkMB, storage.MaxFileSize
}

func (s *UploadService) validateIncomingChunkSize(
	ctx context.Context,
	driver StorageDriver,
	fileHash, index string,
	size int64,
) error {
	if size < 0 {
		return errors.New("分块大小无效")
	}

	chunkMB, fileMB := s.uploadSizeLimits()
	if chunkMB > 0 && size > chunkMB*mebibyte {
		return fmt.Errorf("分块大小超过限制（最大 %dMB)", chunkMB)
	}

	if fileMB <= 0 {
		return nil
	}

	if size > fileMB*mebibyte {
		return fmt.Errorf("文件大小超过限制（最大 %dMB)", fileMB)
	}

	total, current, err := driver.ChunkUploadBytes(ctx, fileHash, index)
	if err != nil {
		return err
	}

	if current > total {
		current = total
	}

	if total-current+size > fileMB*mebibyte {
		return fmt.Errorf("文件大小超过限制（最大 %dMB)", fileMB)
	}

	return nil
}

func (s *UploadService) ensureChunkIntegrity(
	ctx context.Context,
	driver StorageDriver,
	fileHash string,
	totalChunks int,
) error {
	chunks, err := driver.ListUploadedChunks(ctx, fileHash)
	if err != nil {
		return err
	}

	return domain.ValidateChunkSet(chunks, totalChunks, s.storage.ValidateChunkIndex)
}

// ---------------------------------------------------------------------------
// 进程内合并锁
// ---------------------------------------------------------------------------

type mergeLockManager struct {
	mu    sync.Mutex
	locks map[string]*mergeLockEntry
}

type mergeLockEntry struct {
	mu   sync.Mutex
	refs int
}

func newMergeLockManager() *mergeLockManager {
	return &mergeLockManager{locks: make(map[string]*mergeLockEntry)}
}

func (m *mergeLockManager) lock(key string) func() {
	m.mu.Lock()

	entry := m.locks[key]
	if entry == nil {
		entry = &mergeLockEntry{}
		m.locks[key] = entry
	}

	entry.refs++
	m.mu.Unlock()

	entry.mu.Lock()

	return func() {
		entry.mu.Unlock()
		m.mu.Lock()

		entry.refs--
		if entry.refs == 0 {
			delete(m.locks, key)
		}
		m.mu.Unlock()
	}
}

type completedFileProbeResult struct {
	filePath  string
	completed bool
	err       error
}
type uploadedChunksProbeResult struct {
	chunks []string
	err    error
}

func isTransientUploadStatusError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	return false
}

// joinObjectPath 以 / 连接对象路径片段（等价 path.Join 对无冗余分隔符输入的行为）。
func joinObjectPath(parts ...string) string {
	cleaned := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part == "" {
			continue
		}

		cleaned = append(cleaned, part)
	}

	return strings.Join(cleaned, "/")
}
