package storage

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/config/site"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
)

// ReadDASHManifest returns bounded manifest bytes and directory-relative file
// sizes from the configured storage. It never fetches a caller-selected host.
func (r *Resolver) ReadDASHManifest(ctx context.Context, storedPath string, limit int64) ([]byte, map[string]int64, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cfg, err := r.storageConfig()
	if err != nil {
		return nil, nil, err
	}

	read := func(reader io.Reader) ([]byte, error) {
		data, err := io.ReadAll(io.LimitReader(reader, limit+1))
		if err == nil && int64(len(data)) > limit {
			err = errors.New("DASH manifest exceeds size limit")
		}

		return data, err
	}

	var failures []error

	if cfg.Local != nil && cfg.Local.Enable {
		data, sizes, opened, err := r.readLocalDASHManifest(ctx, cfg, storedPath, read)
		if opened {
			return data, sizes, err
		}

		failures = append(failures, err)
	}

	return finishResolverReadDASHManifest(ctx, r, storedPath, cfg, read, failures)
}

func resolverReadDASHManifestVisit(ctx context.Context, name string, entry fs.DirEntry, walkErr error, sizes map[string]int64, root string) error {
	if walkErr != nil {
		return walkErr
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if entry.IsDir() {
		return nil
	}

	if !entry.Type().IsRegular() {
		return nil
	}

	if len(sizes) >= 100000 {
		return errors.New("DASH directory exceeds file limit")
	}

	info, err := entry.Info()
	if err != nil {
		return err
	}

	relative, err := filepath.Rel(root, name)
	if err != nil {
		return err
	}

	sizes[filepath.ToSlash(relative)] = info.Size()

	return nil
}

func finishResolverReadDASHManifest(ctx context.Context, r *Resolver, storedPath string, cfg *site.Storage, read func(reader io.Reader) ([]byte, error), failures []error) ([]byte, map[string]int64, error) {
	if cfg.MinIO == nil || !cfg.MinIO.Enable {
		if len(failures) == 0 {
			return nil, nil, errors.New("DASH storage not configured")
		}

		return nil, nil, fmt.Errorf("DASH manifest not available in configured storage: %w", errors.Join(failures...))
	}

	driver, err := NewMinIOStorage(cfg, r.clients, r.detached, r.logger)
	if err != nil {
		return nil, nil, err
	}

	key, err := driver.objectKeyFromPath(storedPath)
	if err != nil {
		return nil, nil, err
	}

	object, err := driver.client.GetObject(ctx, driver.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, err
	}

	data, err := read(object)
	_ = object.Close()

	if err != nil {
		return nil, nil, err
	}

	prefix := path.Dir(key) + "/"
	sizes := map[string]int64{}

	for entry := range driver.client.ListObjects(ctx, driver.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if entry.Err != nil {
			return nil, nil, entry.Err
		}

		if len(sizes) >= 100000 {
			return nil, nil, errors.New("DASH directory exceeds file limit")
		}

		sizes[strings.TrimPrefix(entry.Key, prefix)] = entry.Size
	}

	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	return data, sizes, nil
}

func (r *Resolver) readLocalDASHManifest(ctx context.Context, cfg *site.Storage, storedPath string, read func(io.Reader) ([]byte, error)) ([]byte, map[string]int64, bool, error) {
	local := NewLocalStorage(cfg, r.logger)

	manifest, err := local.resolveStoredFilePath(storedPath)
	if err != nil {
		return nil, nil, false, err
	}

	file, err := os.Open(manifest) // #nosec G304 -- The manifest path is restricted to the configured local storage root.
	if err != nil {
		return nil, nil, false, err
	}

	data, err := read(file)
	_ = file.Close()

	if err != nil {
		return nil, nil, true, err
	}

	sizes := map[string]int64{}
	root := filepath.Dir(manifest)
	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		return resolverReadDASHManifestVisit(ctx, name, entry, walkErr, sizes, root)
	})

	return data, sizes, true, err
}
