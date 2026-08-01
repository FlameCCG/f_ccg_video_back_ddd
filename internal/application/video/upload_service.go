package video

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// uploadStatusProbeTimeout 上传状态探测的单次超时。
const uploadStatusProbeTimeout = 4 * time.Second

// uploadStatusCacheTTL 「已完成」标记的进程内缓存 TTL。
const uploadStatusCacheTTL = 30 * time.Minute

// ErrUploadStorageUnavailable 存储驱动不可用（原 NewUploadService 失败时的响应文案）。
var ErrUploadStorageUnavailable = errors.New("上传配置错误")

// UploadService 分片上传用例服务（原 service/video_service/upload_service.go）。
//
// 原实现用包级 mergeLockMgr / uploadStatusCacheStore 单例；
// 新架构改为实例字段，由 bootstrap 创建唯一实例注入，语义等价且无包级可变状态。
type UploadService struct {
	storage     StorageResolver
	cfgProvider *config.Provider
	mergeLocks  *mergeLockManager
	statusCache *uploadStatusCache
	logger      *zap.Logger
}

// NewUploadService 构造分片上传用例服务。
func NewUploadService(
	storage StorageResolver,
	cfgProvider *config.Provider,
	logger *zap.Logger,
) *UploadService {
	return &UploadService{
		storage:     storage,
		cfgProvider: cfgProvider,
		mergeLocks:  newMergeLockManager(),
		statusCache: newUploadStatusCache(uploadStatusCacheTTL),
		logger:      logger,
	}
}

// UploadChunk 保存一个分片。
func (s *UploadService) UploadChunk(
	ctx context.Context,
	fileHash, index string,
	size int64,
	reader io.Reader,
) error {
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

	if maxChunkSize := s.maxChunkSizeMB(); maxChunkSize > 0 {
		maxBytes := maxChunkSize * 1024 * 1024
		if size > maxBytes {
			return fmt.Errorf("分块大小超过限制（最大 %dMB)", maxChunkSize)
		}
	}

	return driver.SaveChunk(ctx, fileHash, index, size, reader)
}

// GetUploadedChunks 读取已上传的分片索引列表。
func (s *UploadService) GetUploadedChunks(ctx context.Context, fileHash string) ([]string, error) {
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
	if completedRes.err == nil && completedRes.completed {
		s.statusCache.rememberCompleted(fileHash, completedRes.filePath)
		cancelStatus()

		return &UploadStatus{
			FileHash:       fileHash,
			Completed:      true,
			FilePath:       completedRes.filePath,
			UploadedChunks: []string{},
		}, nil
	}

	// 对象不存在：清掉陈旧 completed 缓存，避免误导前端秒传
	if completedRes.err == nil && !completedRes.completed {
		s.statusCache.forgetCompleted(fileHash)
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
		s.logger.Warn("upload status degraded due to transient storage error",
			zap.String("fileHash", fileHash),
			zap.Error(completedRes.err),
			zap.Error(chunksRes.err))

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
		s.statusCache.rememberCompleted(fileHash, existing)
		s.logger.Info("complete upload reused existing object",
			zap.String("fileHash", fileHash),
			zap.String("filePath", existing))

		return existing, nil
	}

	if err := s.ensureChunkIntegrity(ctx, driver, fileHash, totalChunks); err != nil {
		return "", err
	}

	start := time.Now()

	finalPath, err := driver.MergeChunks(ctx, fileHash, fileName, totalChunks)
	if err != nil {
		s.logger.Warn("complete upload failed",
			zap.String("fileHash", fileHash),
			zap.String("fileName", fileName),
			zap.Int("totalChunks", totalChunks),
			zap.Duration("cost", time.Since(start)),
			zap.Error(err))

		return "", err
	}

	s.logger.Info("complete upload succeeded",
		zap.String("fileHash", fileHash),
		zap.String("fileName", fileName),
		zap.Int("totalChunks", totalChunks),
		zap.Duration("cost", time.Since(start)))
	s.statusCache.rememberCompleted(fileHash, finalPath)

	return finalPath, nil
}

// InvalidateUploadCompletedCache 丢弃陈旧的「已完成」标记，避免源对象被硬删后仍然秒传成功。
func (s *UploadService) InvalidateUploadCompletedCache(fileHash string) {
	s.statusCache.forgetCompleted(trimSpace(fileHash))
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
		s.logger.Warn("resolve completed upload failed", zap.String("fileHash", fileHash), zap.Error(err))

		return "", errCheckUploadFailed
	}

	if !completed || trimSpace(filePath) == "" {
		s.logger.Warn("publish blocked: completed upload object missing",
			zap.String("fileHash", trimSpace(fileHash)))

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
		return nil, ErrUploadStorageUnavailable
	}

	return driver, nil
}

func (s *UploadService) validateFileHash(fileHash string) error {
	return s.storage.ValidateUploadID(trimSpace(fileHash))
}

func (s *UploadService) maxChunkSizeMB() int64 {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil || cfg.Site.Storage == nil {
		return 0
	}

	return cfg.Site.Storage.MaxChunkSize
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
// 进程内合并锁与「已完成」缓存（原包级单例，改为 UploadService 的实例字段）
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

type uploadStatusCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]*uploadStatusCacheEntry
}

type uploadStatusCacheEntry struct {
	updatedAt time.Time
	completed bool
	filePath  string
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

func newUploadStatusCache(ttl time.Duration) *uploadStatusCache {
	if ttl <= 0 {
		ttl = uploadStatusCacheTTL
	}

	return &uploadStatusCache{
		ttl:     ttl,
		entries: make(map[string]*uploadStatusCacheEntry),
	}
}

func (c *uploadStatusCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*uploadStatusCacheEntry)
}

func (c *uploadStatusCache) rememberCompleted(fileHash, filePath string) {
	if fileHash == "" || filePath == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[fileHash] = &uploadStatusCacheEntry{
		updatedAt: time.Now(),
		completed: true,
		filePath:  filePath,
	}
}

func (c *uploadStatusCache) forgetCompleted(fileHash string) {
	if fileHash == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, fileHash)
}

// entryLocked 读取未过期的缓存项（调用方需持有锁）。
func (c *uploadStatusCache) entryLocked(fileHash string) (*uploadStatusCacheEntry, bool) {
	entry, ok := c.entries[fileHash]
	if !ok {
		return nil, false
	}

	if c.ttl > 0 && time.Since(entry.updatedAt) > c.ttl {
		delete(c.entries, fileHash)

		return nil, false
	}

	return entry, true
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

// buildCoverObjectPath 生成封面对象路径（原 api/video_api/upload_cover.go）。
func buildCoverObjectPath(fileHash, ext string, uniqueSuffix string) string {
	ext = strings.TrimSpace(strings.ToLower(ext))
	if ext == "" {
		ext = ".jpg"
	}

	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	return joinObjectPath("images", strings.TrimSpace(fileHash)+"-"+uniqueSuffix+ext)
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
