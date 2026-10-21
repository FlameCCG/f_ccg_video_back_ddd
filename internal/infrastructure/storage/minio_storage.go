package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"

	"f_ccg_video_back_ddd/internal/config/site"
	media "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
)

const (
	defaultChunkPrefix     = "chunks" // 分片对象前缀路径
	minComposePartSize     = 5 * 1024 * 1024
	contentTypeOctetStream = "application/octet-stream"
)

// MinIOStorage 实现存储驱动端口，封装 MinIO 读写逻辑。
type MinIOStorage struct {
	cfg         *site.Storage              // 全量站点存储配置
	client      *minio.Client              // MinIO SDK 客户端
	bucket      string                     // 分片存储所在桶名
	chunkPrefix string                     // 分片对象的前缀路径
	videoPrefix string                     // 合并后视频对象的前缀路径
	detached    *ctxutil.DetachedTaskGroup // 分片清理等 fire-and-forget 任务
	logger      *zap.Logger
}

// 编译期断言：满足领域侧存储驱动端口。
var _ media.StorageDriver = (*MinIOStorage)(nil)

// NewMinIOStorage 根据配置创建 MinIOStorage 实例。
//
// clients 为可复用的客户端缓存；detached 用于分片清理等后台任务（可为 nil，此时同步清理）。
func NewMinIOStorage(
	cfg *site.Storage,
	clients *MinIOClientPool,
	detached *ctxutil.DetachedTaskGroup,
	logger *zap.Logger,
) (*MinIOStorage, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	if cfg == nil || cfg.MinIO == nil { // 未配置 MinIO 时直接报错
		return nil, errors.New("未配置 MinIO 存储")
	}

	if strings.TrimSpace(cfg.MinIO.Bucket) == "" { // 桶名为空不可继续
		return nil, errors.New("未设置 MinIO bucket 名称")
	}

	if clients == nil {
		clients = NewMinIOClientPool(logger)
	}

	client, err := clients.Client(cfg.MinIO) // 懒加载客户端
	if err != nil {
		return nil, err
	}

	if len(cfg.MinIO.PublicPrefixes) > 0 {
		clients.applyPublicPolicy(client, cfg.MinIO)
	}

	return &MinIOStorage{
		cfg:         cfg,
		client:      client,
		bucket:      cfg.MinIO.Bucket,
		chunkPrefix: defaultChunkPrefix,
		videoPrefix: cfg.MinIO.UserUploadPrefix,
		detached:    detached,
		logger:      logger,
	}, nil
}

// SaveChunk 将单个分片写入 MinIO。
func (m *MinIOStorage) SaveChunk(
	ctx context.Context,
	fileHash, index string,
	size int64,
	reader media.ChunkReader,
) error {
	if err := ValidateUploadID(fileHash); err != nil {
		return err
	}

	if err := ValidateChunkIndex(index); err != nil {
		return err
	}

	objectName := m.chunkObjectPath(fileHash, index) // 拼接出对象在桶内的路径
	// 直接覆盖写入分片。断点重传的重复分片由客户端通过 status 接口规避，
	// 服务端不再为主路径额外做 Stat/Remove，减少一次到两次 MinIO 往返。
	_, err := m.client.PutObject(ctx, m.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentTypeOctetStream, // 统一使用二进制类型
	})
	if err != nil {
		// AccessDenied 也可能来自存储服务器的文件系统错误；保留请求 ID 以便关联服务端追踪。
		resp := minio.ToErrorResponse(err)
		m.logger.Error("minio chunk write failed",
			zap.Error(err),
			zap.String("bucket", m.bucket),
			zap.String("key", objectName),
			zap.Int64("size", size),
			zap.String("storageCode", resp.Code),
			zap.Int("storageStatus", resp.StatusCode),
			zap.String("requestID", resp.RequestID))
		return fmt.Errorf("上传分片失败: %w", err)
	}

	return nil
}

// ListUploadedChunks 列出指定文件 hash 已经上传的分片索引。
func (m *MinIOStorage) ListUploadedChunks(ctx context.Context, fileHash string) ([]string, error) {
	if err := ValidateUploadID(fileHash); err != nil {
		return nil, err
	}

	prefix := m.chunkPrefixPath(fileHash) // 获取对应 hash 的前缀路径
	objects := m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	var chunks []string // 用于收集已存在的分片 index

	for object := range objects { // ListObjects 返回 channel，逐个读取
		if object.Err != nil { // 任何一个对象读取失败都需要中止
			return nil, fmt.Errorf("列出分片失败: %w", object.Err)
		}

		if !strings.HasSuffix(object.Key, ".part") { // 非 .part 文件视为无关对象
			continue
		}

		index := strings.TrimSuffix(path.Base(object.Key), ".part")
		if err := ValidateChunkIndex(index); err != nil {
			continue
		}

		chunks = append(chunks, index)
	}

	return chunks, nil
}

// GetCompletedFile 检查最终合并文件是否已存在，并返回可访问 URL。
func (m *MinIOStorage) GetCompletedFile(ctx context.Context, fileHash string) (string, bool, error) {
	if err := ValidateUploadID(fileHash); err != nil {
		return "", false, err
	}

	finalKey := m.videoObjectKey(fileHash)

	if exists, err := m.objectExists(ctx, finalKey); err != nil {
		return "", false, fmt.Errorf("检查最终文件失败: %w", err)
	} else if exists {
		return m.publicPath(finalKey), true, nil
	}

	return "", false, nil
}

// MergeChunks 将顺序分片合并成最终文件并上传回 MinIO。
func (m *MinIOStorage) MergeChunks(ctx context.Context, fileHash, fileName string, totalChunks int) (string, error) {
	if err := ValidateUploadID(fileHash); err != nil {
		return "", err
	}

	if totalChunks <= 0 { // 校验分片数量
		return "", errors.New("分片数量无效")
	}

	finalKey := m.videoObjectKey(fileHash) // 生成最终对象 key

	if exists, err := m.objectExists(ctx, finalKey); err != nil {
		return "", fmt.Errorf("检查最终文件失败: %w", err)
	} else if exists {
		m.cleanupChunksAsync(ctx, fileHash)
		m.logger.Info("complete upload reused existing minio object",
			zap.String("fileHash", fileHash),
			zap.String("key", finalKey))

		return m.publicPath(finalKey), nil
	}

	if ok, err := m.composeChunks(ctx, finalKey, fileHash, fileName, totalChunks); err != nil {
		return "", err
	} else if ok {
		m.cleanupChunksAsync(ctx, fileHash)
		m.logger.Info("complete upload composed chunks on minio",
			zap.String("fileHash", fileHash),
			zap.String("key", finalKey),
			zap.Int("totalChunks", totalChunks))

		return m.publicPath(finalKey), nil
	}

	m.logger.Info("complete upload falling back to stream merge",
		zap.String("fileHash", fileHash),
		zap.String("key", finalKey),
		zap.Int("totalChunks", totalChunks))

	return m.streamMergeChunks(ctx, finalKey, fileHash, fileName, totalChunks)
}

// composeChunks 用服务端 ComposeObject 合并分片；有分片不满 5MiB 时返回 false 让调用方回退流式合并。
func (m *MinIOStorage) composeChunks(
	ctx context.Context,
	finalKey, fileHash, fileName string,
	totalChunks int,
) (bool, error) {
	srcs := make([]minio.CopySrcOptions, 0, totalChunks)

	for i := range totalChunks {
		index := strconv.Itoa(i)
		chunkKey := m.chunkObjectPath(fileHash, index)

		info, err := m.client.StatObject(ctx, m.bucket, chunkKey, minio.StatObjectOptions{})
		if err != nil {
			return false, fmt.Errorf("检查分片 %s 失败: %w", index, err)
		}
		// S3/MinIO Compose 限制：非最后一个源对象至少 5MiB。
		if i < totalChunks-1 && info.Size < minComposePartSize {
			return false, nil
		}

		srcs = append(srcs, minio.CopySrcOptions{
			Bucket: m.bucket,
			Object: chunkKey,
		})
	}

	_, err := m.client.ComposeObject(ctx, minio.CopyDestOptions{
		Bucket:          m.bucket,
		Object:          finalKey,
		ReplaceMetadata: true,
		UserMetadata: map[string]string{
			"Content-Type": guessVideoContentType(fileName),
		},
	}, srcs...)
	if err != nil {
		return false, fmt.Errorf("服务端合并分片失败: %w", err)
	}

	return true, nil
}

// streamMergeChunks 用 io.Pipe 边读边写地把分片流式合并为最终对象。
func (m *MinIOStorage) streamMergeChunks(
	ctx context.Context,
	finalKey, fileHash, fileName string,
	totalChunks int,
) (string, error) {
	pr, pw := io.Pipe() // Pipe 允许边读边写，节省磁盘空间

	go func() {
		defer func() { _ = pw.Close() }()

		for i := range totalChunks {
			index := strconv.Itoa(i)
			chunkKey := m.chunkObjectPath(fileHash, index)

			obj, err := m.client.GetObject(ctx, m.bucket, chunkKey, minio.GetObjectOptions{})
			if err != nil {
				pw.CloseWithError(fmt.Errorf("读取分片 %s 失败: %w", index, err))

				return
			}

			if _, err = io.Copy(pw, obj); err != nil {
				_ = obj.Close()

				pw.CloseWithError(fmt.Errorf("合并分片失败: %w", err))

				return
			}

			_ = obj.Close()
		}
	}()

	_, err := m.client.PutObject(ctx, m.bucket, finalKey, pr, -1, minio.PutObjectOptions{
		ContentType: guessVideoContentType(fileName),
		PartSize:    10 * 1024 * 1024,
	})
	if err != nil {
		return "", fmt.Errorf("写入最终文件失败: %w", err)
	}

	m.cleanupChunksAsync(ctx, fileHash)

	return m.publicPath(finalKey), nil
}

func (m *MinIOStorage) objectExists(ctx context.Context, objectKey string) (bool, error) {
	_, err := m.client.StatObject(ctx, m.bucket, objectKey, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}

	if resp := minio.ToErrorResponse(err); isNotFoundError(resp) {
		return false, nil
	}

	return false, err
}

func guessVideoContentType(fileName string) string {
	ext := strings.ToLower(path.Ext(strings.TrimSpace(fileName)))
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".m3u8":
		return "application/x-mpegURL"
	case ".ts":
		return "video/mp2t"
	case ".mpd":
		return "application/dash+xml"
	case ".m4s":
		return "video/iso.segment"
	default:
		return contentTypeOctetStream
	}
}

// SaveTranscodeVideo MinIO 实现：
// 利用 PutObject 对 size=-1 的支持实现分片流式上传（PartSize 固定 10MB 控制内存占用）。
func (m *MinIOStorage) SaveTranscodeVideo(
	ctx context.Context,
	relativePath string,
	size int64,
	reader io.Reader,
	contentType, label string,
) (string, int64, error) {
	key, err := cleanObjectKey(relativePath)
	if err != nil {
		return "", 0, err
	}

	// 显式配置 PartSize (分片大小)：不设置时 SDK 可能默认分配较大的内存缓冲（如 64MB+）。
	// 这里强制限制为 10MB（S3 协议要求的最小分片通常是 5MB），
	// 使每个并发上传任务只占用约 10MB 内存。
	const partSize = 10 * 1024 * 1024 // 10MB

	opts := minio.PutObjectOptions{
		ContentType: contentType,
		PartSize:    partSize,
	}
	if opts.ContentType == "" {
		opts.ContentType = contentTypeOctetStream
	}

	// 初始化进度条：专门处理 size = -1 的情况
	progress := newProgressLogger(size, label, m.logger)
	defer progress.finish()

	// 使用 TeeReader 包装原始 Reader，在读取数据的同时更新进度
	tracked := io.TeeReader(reader, progress)

	// 当 size 为 -1 时，SDK 会自动进行 Multipart Upload：
	// 读取流直到填满 PartSize (10MB)，然后上传这一片，循环往复。
	info, err := m.client.PutObject(ctx, m.bucket, key, tracked, size, opts)
	if err != nil {
		return "", 0, fmt.Errorf("上传文件失败: %w", err)
	}

	if info.Size <= 0 {
		if removeErr := m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{}); removeErr != nil {
			m.logger.Warn("remove empty transcode object failed", zap.Error(removeErr), zap.String("key", key))
		}

		return "", 0, errors.New("转码输出为空，已拒绝提交")
	}

	return m.publicPath(key), info.Size, nil
}

// StatStoredFile 返回对象字节数；对象缺失时返回 error（转码断点续传用它校验历史产物）。
func (m *MinIOStorage) StatStoredFile(ctx context.Context, storedPath string) (int64, error) {
	key, err := m.objectKeyFromPath(storedPath)
	if err != nil {
		return 0, err
	}

	if key == "" {
		return 0, errors.New("empty object key")
	}

	info, err := m.client.StatObject(ctx, m.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return 0, err
	}

	return info.Size, nil
}

// DeleteStoredFile 删除已存储对象；空路径视为 no-op。
func (m *MinIOStorage) DeleteStoredFile(ctx context.Context, storedPath string) error {
	key, err := m.objectKeyFromPath(storedPath)
	if err != nil {
		return err
	}

	if key == "" {
		// 空路径视为 no-op
		return nil
	}

	// MinIO RemoveObject 如果对象不存在，一般不会报错，
	// 即使报错，这里也直接返回由上层决定要不要忽略。
	return m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
}

// DeleteByPrefix 删除指定前缀下的所有对象，返回删除数量。
func (m *MinIOStorage) DeleteByPrefix(ctx context.Context, prefix string) (int64, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return 0, nil
	}
	// 解析为 object key（去掉 scheme/host/bucket 前缀）
	key, err := m.objectKeyFromPath(prefix)
	if err != nil {
		return 0, err
	}

	if key == "" {
		return 0, nil
	}
	// 确保前缀以 / 结尾，避免误删其他前缀（例如 dash 不会误删 dash2）
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}

	var deleted int64

	for object := range m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{
		Prefix:    key,
		Recursive: true,
	}) {
		if object.Err != nil {
			return deleted, object.Err
		}

		if err := m.client.RemoveObject(ctx, m.bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			return deleted, err
		}

		deleted++
	}

	return deleted, nil
}

func (m *MinIOStorage) objectKeyFromPath(storedPath string) (string, error) {
	trimmed := strings.TrimSpace(storedPath)
	if trimmed == "" {
		return "", nil
	}

	// 如果是 URL，优先解析出 Path
	if u, err := url.Parse(trimmed); err == nil && u.Scheme != "" && u.Host != "" {
		trimmed = u.Path
		if unescaped, err := url.PathUnescape(trimmed); err == nil {
			trimmed = unescaped
		}
	}

	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		return "", nil
	}

	// 如果带 bucket 前缀，去掉它
	if m.bucket != "" {
		if parts := strings.SplitN(trimmed, "/", 2); len(parts) == 2 && parts[0] == m.bucket {
			trimmed = parts[1]
		} else if trimmed == m.bucket {
			// 只有 bucket 名，没有对象 key
			return "", nil
		}
	}
	// S3/MinIO key 使用 path.Clean 清理（始终用 /）
	key, err := cleanObjectKey(trimmed)
	if err != nil {
		return "", err
	}

	if key == "." {
		return "", nil
	}

	return key, nil
}

// cleanupChunksAsync 以 detached 任务清理分片；未注入任务组时退化为同步清理。
func (m *MinIOStorage) cleanupChunksAsync(parent context.Context, fileHash string) {
	if m.detached == nil {
		m.cleanupChunks(parent, fileHash)

		return
	}

	if !m.detached.Go(parent, ctxutil.AsyncLongTimeout, func(ctx context.Context) {
		m.cleanupChunks(ctx, fileHash)
	}) {
		m.logger.Warn("skip minio chunk cleanup: process is shutting down", zap.String("fileHash", fileHash))
	}
}

// cleanupChunks 删除某个文件 hash 下所有已上传分片。
func (m *MinIOStorage) cleanupChunks(ctx context.Context, fileHash string) {
	if err := ValidateUploadID(fileHash); err != nil {
		m.logger.Warn("skip cleanup chunks with invalid fileHash", zap.Error(err))

		return
	}

	prefix := m.chunkPrefixPath(fileHash) // 获取需要清理的目录
	for object := range m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if object.Err != nil { // 如果列举失败，记录日志并结束
			m.logger.Warn("list chunk objects failed", zap.Error(object.Err))

			return
		}

		if err := m.client.RemoveObject(ctx, m.bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			m.logger.Warn("remove chunk object failed", zap.Error(err), zap.String("key", object.Key))
		}
	}
}

// chunkObjectPath 生成具体分片在桶中的路径。
func (m *MinIOStorage) chunkObjectPath(fileHash, index string) string {
	filename := index + ".part" // 分片文件统一以 .part 结尾

	return path.Join(m.chunkPrefix, fileHash, filename)
}

// chunkPrefixPath 返回某个 hash 的目录前缀。
func (m *MinIOStorage) chunkPrefixPath(fileHash string) string {
	return path.Join(m.chunkPrefix, fileHash)
}

// videoObjectKey 返回合并后源视频的对象 key。
func (m *MinIOStorage) videoObjectKey(fileHash string) string {
	return path.Join(m.videoPrefix, fileHash, "_source_"+fileHash+".mp4")
}

// SaveFile 将任意文件上传到指定对象路径。
func (m *MinIOStorage) SaveFile(
	ctx context.Context,
	relativePath string,
	size int64,
	reader io.Reader,
	contentType string,
) (string, error) {
	key, err := cleanObjectKey(relativePath)
	if err != nil {
		return "", err
	}

	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}
	if opts.ContentType == "" {
		opts.ContentType = contentTypeOctetStream
	}

	_, err = m.client.PutObject(ctx, m.bucket, key, reader, size, opts)
	if err != nil {
		return "", fmt.Errorf("上传文件失败: %w", err)
	}

	return m.publicPath(key), nil
}

// publicPath 根据对象 key 生成根相对访问路径（host 由前端按环境拼接，见 docs）。
// 对外返回 /<key>，例如 /user_files/images/x.jpg；缺少配置时原样返回 key。
func (m *MinIOStorage) publicPath(objectKey string) string {
	key := strings.TrimSpace(objectKey)
	if key == "" {
		return ""
	}

	return "/" + strings.TrimPrefix(key, "/")
}

// absoluteURL 根据 endpoint、bucket 和对象 key 生成后端直连的完整 URL（ffmpeg 等内部消费）。
func (m *MinIOStorage) absoluteURL(objectKey string) string {
	if m.cfg == nil || m.cfg.MinIO == nil { // 缺少配置时直接返回 key
		return objectKey
	}

	scheme := "http"        // 默认使用 http
	if m.cfg.MinIO.UseSSL { // 若开启 SSL 则使用 https
		scheme = "https"
	}

	endpoint := strings.TrimPrefix(m.cfg.MinIO.Endpoint, "https://") // 去掉协议前缀，防止重复
	endpoint = strings.TrimPrefix(endpoint, "http://")

	return fmt.Sprintf("%s://%s/%s/%s", scheme, endpoint, m.bucket, strings.TrimPrefix(objectKey, "/"))
}

// ResolveSource 把根相对路径解析为后端直连的完整 URL；已是 http(s) URL 时原样返回。
func (m *MinIOStorage) ResolveSource(_ context.Context, storedPath string) (string, error) {
	trimmed := strings.TrimSpace(storedPath)
	if trimmed == "" {
		return "", nil
	}

	low := strings.ToLower(trimmed)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return trimmed, nil
	}

	key, err := m.objectKeyFromPath(trimmed)
	if err != nil {
		return "", err
	}

	if key == "" {
		return "", nil
	}

	return m.absoluteURL(key), nil
}

// isNotFoundError 判断错误是否为对象不存在类型。
func isNotFoundError(resp minio.ErrorResponse) bool {
	switch resp.Code {
	case "NoSuchKey", "NoSuchObject", "NoSuchEntity": // MinIO 可能返回这些 code 之一
		return true
	default:
		return false
	}
}
