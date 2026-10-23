// Package storage 实现本地磁盘与 MinIO 存储驱动。
//
// StorageDriver 端口位于 domain/media，Resolver 按当前配置或显式存储类型选择实现。
// local_storage.go/minio_storage.go 对齐分片、合并、探测、流式写入和删除能力；safety.go 约束路径与上传标识。
// 数据库中的相对路径/对象 key、浏览器访问 URL 和 FFmpeg 可读源路径是不同表示，转换要经过驱动。
// upload_garbage.go 只扫描和删除存储候选，是否仍被数据库引用及并发协调由上传清理用例决定。
//
// 具体对象由 bootstrap 构造并通过端口交给内层，技术实现不应反过来要求业务层导入本包。
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/config/site"
	media "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

// LocalStorage 基于本地磁盘的分片上传与合并存储驱动。
type LocalStorage struct {
	cfg    *site.Storage
	logger *zap.Logger
}

// 编译期断言：满足领域侧存储驱动端口。
var _ media.StorageDriver = (*LocalStorage)(nil)

// NewLocalStorage 创建本地存储驱动实例。
func NewLocalStorage(cfg *site.Storage, logger *zap.Logger) *LocalStorage {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &LocalStorage{cfg: cfg, logger: logger}
}

// SaveChunk 把单个分片写入本地分块目录（先写 .uploading 临时文件再原子 rename）。
func (l *LocalStorage) SaveChunk(
	_ context.Context,
	fileHash, index string,
	size int64,
	reader media.ChunkReader,
) error {
	if l.cfg == nil || l.cfg.ChunkDir == "" {
		return errors.New("未设置本地分块目录")
	}

	if err := ValidateUploadID(fileHash); err != nil {
		return err
	}

	if err := ValidateChunkIndex(index); err != nil {
		return err
	}

	dir, err := safeJoinLocal(l.cfg.ChunkDir, fileHash)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建分块目录失败: %w", err)
	}

	chunkPath := filepath.Join(dir, index+".part")
	if stat, statErr := os.Stat(chunkPath); statErr == nil {
		if stat.Size() == size {
			// 同大小续传虽无需重写内容，仍视为一次成功分片请求。
			return os.Chtimes(chunkPath, time.Now(), time.Now())
		}

		if err := os.Remove(chunkPath); err != nil {
			return fmt.Errorf("删除旧的(损坏的)分块失败: %w", err)
		}
	}

	tmpPath := chunkPath + ".uploading"

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建分块文件失败: %w", err)
	}

	closed := false
	defer func() {
		if !closed {
			if err := out.Close(); err != nil {
				l.logger.Warn("close temp chunk file failed", zap.Error(err), zap.String("path", tmpPath))
			}
		}
		_ = os.Remove(tmpPath)
	}()

	if _, err = io.Copy(out, reader); err != nil {
		// 如果写入失败（例如客户端断开连接、磁盘已满），
		// 必须删除这个刚创建的、不完整的临时文件，防止留下垃圾数据。
		_ = os.Remove(tmpPath)

		return fmt.Errorf("写入分块数据失败: %w", err)
	}

	if err = out.Sync(); err != nil {
		_ = os.Remove(tmpPath)

		return fmt.Errorf("同步分块数据失败: %w", err)
	}
	closeErr := out.Close()
	closed = true
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("关闭分块文件失败: %w", closeErr)
	}

	if err = os.Rename(tmpPath, chunkPath); err != nil {
		_ = os.Remove(tmpPath)

		return fmt.Errorf("保存分块失败: %w", err)
	}

	return nil
}

// ListUploadedChunks 列出指定文件 hash 已上传的分片索引。
func (l *LocalStorage) ListUploadedChunks(_ context.Context, fileHash string) ([]string, error) {
	if err := ValidateUploadID(fileHash); err != nil {
		return nil, err
	}

	dir, err := safeJoinLocal(l.cfg.ChunkDir, fileHash)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取分块目录失败: %w", err)
	}

	chunks := make([]string, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".part" {
			continue
		}

		index := name[:len(name)-len(".part")]
		if err := ValidateChunkIndex(index); err != nil {
			continue
		}

		chunks = append(chunks, index)
	}

	return chunks, nil
}

// GetCompletedFile 检查本地最终文件是否已存在，并返回根相对访问路径。
func (l *LocalStorage) GetCompletedFile(_ context.Context, fileHash string) (string, bool, error) {
	if l.cfg == nil || l.cfg.Local == nil || strings.TrimSpace(l.cfg.Local.Path) == "" {
		return "", false, errors.New("未配置本地文件存储路径")
	}

	if err := ValidateUploadID(fileHash); err != nil {
		return "", false, err
	}

	rel := path.Join("videos", fileHash, "_source_"+fileHash+".mp4")

	finalPath, err := safeJoinLocal(l.cfg.Local.Path, rel)
	if err != nil {
		return "", false, err
	}

	_, err = os.Stat(finalPath)
	if err == nil {
		return l.publicPath(rel), true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}

	return "", false, fmt.Errorf("检查最终文件失败: %w", err)
}

// MergeChunks 顺序拼接分片为最终文件（先写 .merging 临时文件再原子 rename）。
func (l *LocalStorage) MergeChunks(ctx context.Context, fileHash, _ string, totalChunks int) (string, error) {
	if err := ValidateUploadID(fileHash); err != nil {
		return "", err
	}

	chunkDir, err := safeJoinLocal(l.cfg.ChunkDir, fileHash)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(chunkDir); os.IsNotExist(err) {
		return "", errors.New("文件分片不存在，请重新上传")
	}

	if l.cfg.Local == nil {
		return "", errors.New("未配置本地文件存储路径")
	}

	rel := path.Join("videos", fileHash, "_source_"+fileHash+".mp4")

	finalDir, err := safeJoinLocal(l.cfg.Local.Path, "videos", fileHash)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(finalDir, 0o755); err != nil {
		return "", fmt.Errorf("创建文件目录失败: %w", err)
	}

	finalName := "_source_" + fileHash + ".mp4"
	finalPath := filepath.Join(finalDir, finalName)
	tmpFinalPath := finalPath + ".merging"

	// 如果最终文件已经存在，则直接返回路径
	if _, err := os.Stat(finalPath); err == nil {
		go func() {
			if err := os.RemoveAll(chunkDir); err != nil {
				l.logger.Warn("clean chunk dir failed", zap.Error(err), zap.String("dir", chunkDir))
			}
		}()

		return l.publicPath(rel), nil
	}

	out, err := os.Create(tmpFinalPath)
	if err != nil {
		return "", fmt.Errorf("创建最终文件失败: %w", err)
	}

	closed := false
	defer func() {
		if !closed {
			if err := out.Close(); err != nil {
				l.logger.Warn("close final file failed", zap.Error(err), zap.String("path", tmpFinalPath))
			}
		}
		_ = os.Remove(tmpFinalPath)
	}()

	for i := range totalChunks {
		if err := ctx.Err(); err != nil {
			_ = os.Remove(tmpFinalPath)

			return "", err
		}

		indexStr := strconv.Itoa(i)
		chunkPath := filepath.Join(chunkDir, indexStr+".part")

		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			_ = os.Remove(tmpFinalPath)

			return "", fmt.Errorf("打开分片文件失败: %w", err)
		}

		if _, err = io.Copy(out, chunkFile); err != nil {
			_ = chunkFile.Close()
			_ = os.Remove(tmpFinalPath)

			return "", fmt.Errorf("写入分片数据失败: %w", err)
		}

		if err := chunkFile.Close(); err != nil {
			_ = os.Remove(tmpFinalPath)

			return "", fmt.Errorf("关闭分片文件失败: %w", err)
		}
	}
	// 数据落盘
	if err = out.Sync(); err != nil {
		_ = os.Remove(tmpFinalPath)

		return "", fmt.Errorf("合并最终文件落盘失败%w", err)
	}
	closeErr := out.Close()
	closed = true
	if closeErr != nil {
		_ = os.Remove(tmpFinalPath)
		return "", fmt.Errorf("关闭最终文件失败: %w", closeErr)
	}

	if err = os.Rename(tmpFinalPath, finalPath); err != nil {
		_ = os.Remove(tmpFinalPath)

		return "", fmt.Errorf("提交最终文件失败: %w", err)
	}

	go func() {
		if cleanErr := os.RemoveAll(chunkDir); cleanErr != nil {
			l.logger.Warn("clean chunk dir failed", zap.Error(cleanErr), zap.String("dir", chunkDir))
		}
	}()

	return l.publicPath(rel), nil
}

// SaveFile 将任意文件写入本地存储。
func (l *LocalStorage) SaveFile(
	_ context.Context,
	relativePath string,
	_ int64,
	reader io.Reader,
	_ string,
) (string, error) {
	if l.cfg == nil || l.cfg.Local == nil {
		return "", errors.New("未配置本地存储")
	}

	destPath, err := safeJoinLocal(l.cfg.Local.Path, relativePath)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}

	defer func() {
		if closeErr := out.Close(); closeErr != nil {
			l.logger.Warn("close saved file failed", zap.Error(closeErr), zap.String("path", destPath))
		}
	}()

	if _, err = io.Copy(out, reader); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	if err = out.Sync(); err != nil {
		return "", fmt.Errorf("落盘失败: %w", err)
	}

	return l.publicPath(relativePath), nil
}

// SaveTranscodeVideo 本地存储实现：
// 利用 io.Copy 持续从 reader（通常是 ffmpeg 产物文件）读取数据写入文件，直到 EOF。
func (l *LocalStorage) SaveTranscodeVideo(
	_ context.Context,
	relativePath string,
	size int64,
	reader io.Reader,
	_, label string,
) (string, int64, error) {
	if l.cfg == nil || l.cfg.Local == nil {
		return "", 0, errors.New("未配置本地存储")
	}

	// 防止路径穿越，确保安全
	destPath, err := safeJoinLocal(l.cfg.Local.Path, relativePath)
	if err != nil {
		return "", 0, err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return "", 0, fmt.Errorf("创建目录失败: %w", err)
	}

	// 创建目标文件
	out, err := os.Create(destPath)
	if err != nil {
		return "", 0, fmt.Errorf("创建文件失败: %w", err)
	}

	closed := false

	// 使用 defer 关闭文件，确保资源释放（否则文件句柄泄露）。
	defer func() {
		if closed {
			return
		}

		if cerr := out.Close(); cerr != nil {
			l.logger.Warn("close transcode file failed", zap.Error(cerr), zap.String("path", destPath))
		}
	}()

	progress := newProgressLogger(size, label, l.logger)
	defer progress.finish()

	tee := io.TeeReader(reader, progress)

	// io.Copy 会阻塞直到 reader 返回 EOF
	written, err := io.Copy(out, tee)
	if err != nil {
		return "", 0, fmt.Errorf("写入文件失败: %w", err)
	}

	if written <= 0 {
		if closeErr := out.Close(); closeErr != nil {
			return "", 0, fmt.Errorf("关闭空转码文件失败: %w", closeErr)
		}

		closed = true

		if removeErr := os.Remove(destPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			l.logger.Warn("remove empty transcode file failed", zap.Error(removeErr), zap.String("path", destPath))
		}

		return "", 0, errors.New("转码输出为空，已拒绝提交")
	}

	// 强制落盘，确保数据物理写入
	if err = out.Sync(); err != nil {
		return "", 0, fmt.Errorf("落盘失败: %w", err)
	}

	return l.publicPath(relativePath), written, nil
}

// resolveStoredFilePath 把存储路径/URL 解析为 base 目录内的绝对路径（含目录穿越防护）。
// 返回空路径且无错误表示 storedPath 只指向根目录。
func (l *LocalStorage) resolveStoredFilePath(storedPath string) (string, error) {
	trimmed := strings.TrimSpace(storedPath)
	if trimmed == "" {
		return "", nil
	}

	if l.cfg == nil || l.cfg.Local == nil || l.cfg.Local.Path == "" {
		return "", errors.New("local storage not configured")
	}
	// 如果是 URL，提取 path 部分
	if u, err := url.Parse(trimmed); err == nil && u.Scheme != "" && u.Host != "" {
		trimmed = u.Path
	}
	// 去掉开头的 /
	trimmed = strings.TrimPrefix(trimmed, "/")

	// 根相对路径含存储根前缀（/uploads/...）时剥掉，避免与 base 重复拼接。
	if root := strings.Trim(filepath.ToSlash(strings.TrimSpace(l.cfg.Local.Path)), "/"); root != "" {
		if rest, ok := strings.CutPrefix(trimmed, root+"/"); ok {
			trimmed = rest
		} else if trimmed == root {
			trimmed = ""
		}
	}

	cleaned := filepath.Clean(trimmed)
	if cleaned == "" || cleaned == "." {
		return "", nil
	}

	base, err := filepath.Abs(filepath.Clean(l.cfg.Local.Path))
	if err != nil {
		return "", err
	}

	fullPath := cleaned
	if !filepath.IsAbs(cleaned) {
		fullPath = filepath.Join(base, cleaned)
	}

	fullPath, err = filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	// 防止目录穿越：fullPath 必须在 base 之内
	rel, err := filepath.Rel(base, fullPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid storedPath %q: outside of base dir", storedPath)
	}

	return fullPath, nil
}

// ResolveSource 把根相对路径解析为本地绝对路径；已是绝对路径时原样返回。
//
// 此入口面向仓储保存的源路径和内部绝对路径；当前实现先判断 filepath.IsAbs。
// 在类 Unix 系统中 /uploads/... 同样会被视为绝对路径，不能仅凭浏览器 URL 形式推断一定经过存储根目录拼接。
func (l *LocalStorage) ResolveSource(_ context.Context, storedPath string) (string, error) {
	trimmed := strings.TrimSpace(storedPath)
	if trimmed == "" {
		return "", nil
	}

	if filepath.IsAbs(trimmed) {
		return trimmed, nil
	}

	return l.resolveStoredFilePath(trimmed)
}

// publicPath 生成根相对访问路径：/<Local.Path>/<relativePath>（host 由前端按环境拼接）。
func (l *LocalStorage) publicPath(relativePath string) string {
	rel := path.Clean("/" + filepath.ToSlash(strings.TrimSpace(relativePath)))
	rel = strings.TrimPrefix(rel, "/")

	if l.cfg == nil || l.cfg.Local == nil {
		return "/" + rel
	}

	root := strings.Trim(filepath.ToSlash(strings.TrimSpace(l.cfg.Local.Path)), "/")
	if root == "" {
		return "/" + rel
	}

	return "/" + path.Join(root, rel)
}

// StatStoredFile 返回本地存储文件的字节数；文件缺失时返回 error（转码断点续传用它校验历史产物）。
func (l *LocalStorage) StatStoredFile(_ context.Context, storedPath string) (int64, error) {
	fullPath, err := l.resolveStoredFilePath(storedPath)
	if err != nil {
		return 0, err
	}

	if fullPath == "" {
		return 0, fmt.Errorf("invalid storedPath %q", storedPath)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return 0, err
	}

	if info.IsDir() {
		return 0, fmt.Errorf("storedPath %q is a directory", storedPath)
	}

	return info.Size(), nil
}

// DeleteStoredFile 删除本地存储文件；空路径或只指向根目录时视为 no-op。
func (l *LocalStorage) DeleteStoredFile(_ context.Context, storedPath string) error {
	fullPath, err := l.resolveStoredFilePath(storedPath)
	if err != nil {
		return err
	}

	if fullPath == "" {
		// 空路径或只指向根目录，不删
		return nil
	}

	if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

// DeleteByPrefix 删除指定前缀（目录）下的所有文件，返回删除数量。
func (l *LocalStorage) DeleteByPrefix(_ context.Context, prefix string) (int64, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return 0, nil
	}

	if l.cfg == nil || l.cfg.Local == nil || l.cfg.Local.Path == "" {
		return 0, errors.New("local storage not configured")
	}
	// 解析为本地路径
	cleaned := prefix
	if u, err := url.Parse(prefix); err == nil && u.Scheme != "" && u.Host != "" {
		cleaned = u.Path
	}

	cleaned = strings.TrimPrefix(cleaned, "/")

	cleaned = filepath.Clean(cleaned)
	if cleaned == "" || cleaned == "." {
		return 0, nil
	}

	base, err := filepath.Abs(filepath.Clean(l.cfg.Local.Path))
	if err != nil {
		return 0, err
	}

	dirPath := cleaned
	if !filepath.IsAbs(cleaned) {
		dirPath = filepath.Join(base, cleaned)
	}

	dirPath, err = filepath.Abs(dirPath)
	if err != nil {
		return 0, err
	}
	// 防止目录穿越
	rel, err := filepath.Rel(base, dirPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return 0, fmt.Errorf("invalid prefix %q: outside of base dir", prefix)
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}

		return 0, err
	}

	var deleted int64

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return deleted, err
		}

		deleted++
	}
	// 删除目录本身（已空）
	_ = os.Remove(dirPath)

	return deleted, nil
}
