package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func atomicWriteFile(destination string, reader io.Reader, requireContent bool) (int64, error) {
	out, err := os.CreateTemp(filepath.Dir(destination), ".upload-*")
	if err != nil {
		return 0, err
	}
	temp := out.Name()
	defer func() { _ = out.Close(); _ = os.Remove(temp) }()
	written, err := io.Copy(out, reader)
	if err != nil {
		return 0, fmt.Errorf("write file: %w", err)
	}
	if requireContent && written == 0 {
		return 0, fmt.Errorf("empty transcode output")
	}
	if err := out.Sync(); err != nil {
		return 0, err
	}
	if err := out.Close(); err != nil {
		return 0, err
	}
	if err := atomicRename(temp, destination); err != nil {
		return 0, err
	}
	return written, nil
}

// Windows may briefly deny replacement while another rename/read closes its handle.
// Retry the atomic operation; never unlink the destination to work around sharing violations.
func atomicRename(source, destination string) error {
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		err = os.Rename(source, destination)
		if err == nil || runtime.GOOS != "windows" || !errors.Is(err, os.ErrPermission) {
			return err
		}
		time.Sleep(time.Millisecond * time.Duration(1<<attempt))
	}
	return err
}
