package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/config/site"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
)

// UploadGarbageScanner 直接遍历存储目录和对象前缀，不依赖新写入的索引，因而覆盖历史遗留物。
type UploadGarbageScanner struct {
	config  *infraconfig.Provider
	clients *MinIOClientPool
	logger  *zap.Logger
}

var _ app.UploadGarbageScanner = (*UploadGarbageScanner)(nil)

func NewUploadGarbageScanner(config *infraconfig.Provider, clients *MinIOClientPool, logger *zap.Logger) *UploadGarbageScanner {
	return &UploadGarbageScanner{config: config, clients: clients, logger: logger}
}

func (s *UploadGarbageScanner) storageConfig() *site.Storage {
	if s == nil || s.config == nil {
		return nil
	}
	cfg := s.config.Get()
	if cfg == nil || cfg.Site == nil {
		return nil
	}
	return cfg.Site.Storage
}

func (s *UploadGarbageScanner) List(ctx context.Context, chunkBefore, sourceBefore time.Time) ([]app.UploadGarbage, error) {
	cfg := s.storageConfig()
	if cfg == nil {
		return nil, errors.New("storage configuration missing")
	}
	var out []app.UploadGarbage
	var scanErr error
	if cfg.ChunkDir != "" || cfg.Local != nil {
		local, err := s.listLocal(ctx, cfg, chunkBefore, sourceBefore)
		out = append(out, local...)
		scanErr = errors.Join(scanErr, err)
	}
	if cfg.MinIO != nil && cfg.MinIO.Endpoint != "" && cfg.MinIO.Bucket != "" {
		remote, err := s.listMinIO(ctx, cfg, chunkBefore, sourceBefore)
		out = append(out, remote...)
		scanErr = errors.Join(scanErr, err)
	}
	return out, scanErr
}

func (s *UploadGarbageScanner) listLocal(ctx context.Context, cfg *site.Storage, chunkBefore, sourceBefore time.Time) ([]app.UploadGarbage, error) {
	var out []app.UploadGarbage
	if cfg.ChunkDir != "" {
		entries, err := readDirIfExists(cfg.ChunkDir)
		if err != nil {
			return nil, fmt.Errorf("list local chunks: %w", err)
		}
		for _, entry := range entries {
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			if !entry.IsDir() || ValidateUploadID(entry.Name()) != nil {
				continue
			}
			dir, err := safeJoinLocal(cfg.ChunkDir, entry.Name())
			if err != nil {
				continue
			}
			latest, err := latestLocalChunk(dir)
			if err != nil {
				return out, err
			}
			if !latest.IsZero() && !latest.After(chunkBefore) {
				out = append(out, app.UploadGarbage{Kind: app.UploadGarbageChunks, Location: app.UploadGarbageLocal,
					FileHash: entry.Name(), Key: entry.Name(), Root: cfg.ChunkDir, LastModified: latest})
			}
		}
	}
	if cfg.Local == nil || cfg.Local.Path == "" {
		return out, nil
	}
	root := filepath.Join(cfg.Local.Path, "videos")
	entries, err := readDirIfExists(root)
	if err != nil {
		return out, fmt.Errorf("list local sources: %w", err)
	}
	driver := NewLocalStorage(cfg, s.logger)
	for _, entry := range entries {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		hash := entry.Name()
		if !entry.IsDir() || ValidateUploadID(hash) != nil {
			continue
		}
		rel := path.Join("videos", hash, "_source_"+hash+".mp4")
		full, err := safeJoinLocal(cfg.Local.Path, rel)
		if err != nil {
			continue
		}
		info, err := os.Lstat(full)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return out, err
		}
		if !info.Mode().IsRegular() || info.ModTime().After(sourceBefore) {
			continue
		}
		out = append(out, app.UploadGarbage{Kind: app.UploadGarbageSource, Location: app.UploadGarbageLocal,
			FileHash: hash, Key: rel, Root: cfg.Local.Path, SourcePath: driver.publicPath(rel), LastModified: info.ModTime()})
	}
	return out, nil
}

func readDirIfExists(root string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return entries, err
}

// Only committed .part files count as successful writes. A directory with no part uses
// its most recent temporary file/directory timestamp to collect abandoned attempts.
func latestLocalChunk(dir string) (time.Time, error) {
	entries, err := readDirIfExists(dir)
	if err != nil {
		return time.Time{}, err
	}
	var latest, fallback time.Time
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return time.Time{}, err
		}
		if info.ModTime().After(fallback) {
			fallback = info.ModTime()
		}
		if strings.HasSuffix(entry.Name(), ".part") {
			index := strings.TrimSuffix(entry.Name(), ".part")
			if ValidateChunkIndex(index) == nil && info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
	}
	if !latest.IsZero() {
		return latest, nil
	}
	if !fallback.IsZero() {
		return fallback, nil
	}
	info, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func (s *UploadGarbageScanner) minioDriver(cfg *site.Storage) (*MinIOStorage, error) {
	return NewMinIOStorage(cfg, s.clients, nil, s.logger)
}

func minioUploadRoot(cfg *site.Storage) string {
	if cfg == nil || cfg.MinIO == nil {
		return ""
	}
	return fmt.Sprintf("%t|%s|%s", cfg.MinIO.UseSSL, cfg.MinIO.Endpoint, cfg.MinIO.Bucket)
}

func (s *UploadGarbageScanner) listMinIO(ctx context.Context, cfg *site.Storage, chunkBefore, sourceBefore time.Time) ([]app.UploadGarbage, error) {
	m, err := s.minioDriver(cfg)
	if err != nil {
		return nil, err
	}
	var out []app.UploadGarbage
	lastChunk := make(map[string]time.Time)
	for obj := range m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{Prefix: m.chunkPrefix + "/", Recursive: true}) {
		if obj.Err != nil {
			return out, fmt.Errorf("list minio chunks: %w", obj.Err)
		}
		parts := strings.Split(strings.TrimPrefix(obj.Key, m.chunkPrefix+"/"), "/")
		if len(parts) != 2 || ValidateUploadID(parts[0]) != nil || !strings.HasSuffix(parts[1], ".part") {
			continue
		}
		if ValidateChunkIndex(strings.TrimSuffix(parts[1], ".part")) != nil {
			continue
		}
		if obj.LastModified.After(lastChunk[parts[0]]) {
			lastChunk[parts[0]] = obj.LastModified
		}
	}
	for hash, latest := range lastChunk {
		if latest.After(chunkBefore) {
			continue
		}
		out = append(out, app.UploadGarbage{Kind: app.UploadGarbageChunks, Location: app.UploadGarbageMinIO,
			FileHash: hash, Key: m.chunkPrefixPath(hash), Root: minioUploadRoot(cfg), LastModified: latest})
	}
	prefix := strings.Trim(m.videoPrefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	for obj := range m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return out, fmt.Errorf("list minio sources: %w", obj.Err)
		}
		hash := path.Base(path.Dir(obj.Key))
		if ValidateUploadID(hash) != nil || obj.Key != m.videoObjectKey(hash) || obj.LastModified.After(sourceBefore) {
			continue
		}
		out = append(out, app.UploadGarbage{Kind: app.UploadGarbageSource, Location: app.UploadGarbageMinIO,
			FileHash: hash, Key: obj.Key, Root: minioUploadRoot(cfg), SourcePath: m.publicPath(obj.Key), LastModified: obj.LastModified})
	}
	return out, nil
}

func (s *UploadGarbageScanner) DeleteIfExpired(ctx context.Context, item app.UploadGarbage, before time.Time) (bool, error) {
	if ValidateUploadID(item.FileHash) != nil {
		return false, errors.New("invalid upload hash")
	}
	cfg := s.storageConfig()
	if cfg == nil {
		return false, errors.New("storage configuration missing")
	}
	switch item.Location {
	case app.UploadGarbageLocal:
		return s.deleteLocal(ctx, cfg, item, before)
	case app.UploadGarbageMinIO:
		return s.deleteMinIO(ctx, cfg, item, before)
	default:
		return false, errors.New("invalid upload garbage location")
	}
}

func (s *UploadGarbageScanner) deleteLocal(ctx context.Context, cfg *site.Storage, item app.UploadGarbage, before time.Time) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if item.Kind == app.UploadGarbageChunks {
		if cfg.ChunkDir == "" || cfg.ChunkDir != item.Root || item.Key != item.FileHash {
			return false, nil
		}
		dir, err := safeJoinLocal(cfg.ChunkDir, item.FileHash)
		if err != nil {
			return false, err
		}
		latest, err := latestLocalChunk(dir)
		if err != nil || latest.IsZero() || latest.After(before) {
			return false, err
		}
		return true, os.RemoveAll(dir)
	}
	if item.Kind != app.UploadGarbageSource || cfg.Local == nil || cfg.Local.Path != item.Root {
		return false, nil
	}
	rel := path.Join("videos", item.FileHash, "_source_"+item.FileHash+".mp4")
	if item.Key != rel {
		return false, nil
	}
	driver := NewLocalStorage(cfg, s.logger)
	if item.SourcePath != driver.publicPath(rel) {
		return false, nil
	}
	full, err := safeJoinLocal(cfg.Local.Path, rel)
	if err != nil {
		return false, err
	}
	info, err := os.Lstat(full)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.ModTime().After(before) {
		return false, nil
	}
	if err := os.Remove(full); err != nil {
		return false, err
	}
	return true, nil
}

func (s *UploadGarbageScanner) deleteMinIO(ctx context.Context, cfg *site.Storage, item app.UploadGarbage, before time.Time) (bool, error) {
	if cfg.MinIO == nil || minioUploadRoot(cfg) != item.Root {
		return false, nil
	}
	m, err := s.minioDriver(cfg)
	if err != nil {
		return false, err
	}
	if item.Kind == app.UploadGarbageChunks {
		prefix := m.chunkPrefixPath(item.FileHash)
		if item.Key != prefix {
			return false, nil
		}
		var keys []string
		var latest time.Time
		for obj := range m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if obj.Err != nil {
				return false, obj.Err
			}
			keys = append(keys, obj.Key)
			if obj.LastModified.After(latest) {
				latest = obj.LastModified
			}
		}
		if len(keys) == 0 || latest.After(before) {
			return false, nil
		}
		for _, key := range keys {
			if err := m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{}); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	if item.Kind != app.UploadGarbageSource || item.Key != m.videoObjectKey(item.FileHash) || item.SourcePath != m.publicPath(item.Key) {
		return false, nil
	}
	info, err := m.client.StatObject(ctx, m.bucket, item.Key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFoundError(minio.ToErrorResponse(err)) {
			return false, nil
		}
		return false, err
	}
	if info.LastModified.After(before) {
		return false, nil
	}
	if err := m.client.RemoveObject(ctx, m.bucket, item.Key, minio.RemoveObjectOptions{}); err != nil {
		return false, err
	}
	return true, nil
}
