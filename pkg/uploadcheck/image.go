// Package uploadcheck provides package functionality.
package uploadcheck

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

const (
	// DefaultMaxImageBytes 常量.
	DefaultMaxImageBytes = 10 * 1024 * 1024
)

var imageExtensionsByMIME = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// ValidateImage 校验图片大小和真实 MIME，并把 reader 复位到开头。
func ValidateImage(reader io.ReadSeeker, filename string, size, maxBytes int64) (ext, contentType string, err error) {
	if reader == nil {
		return "", "", errors.New("图片不能为空")
	}

	if maxBytes <= 0 {
		maxBytes = DefaultMaxImageBytes
	}

	if size <= 0 {
		return "", "", errors.New("图片不能为空")
	}

	if size > maxBytes {
		return "", "", fmt.Errorf("图片大小不能超过 %dMB", maxBytes/(1024*1024))
	}

	head := make([]byte, 512)

	n, readErr := reader.Read(head)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", "", errors.New("读取图片失败")
	}

	if _, err = reader.Seek(0, io.SeekStart); err != nil {
		return "", "", errors.New("读取图片失败")
	}

	contentType = http.DetectContentType(head[:n])

	detectedExt, ok := imageExtensionsByMIME[contentType]
	if !ok {
		return "", "", errors.New("仅支持 jpg、png、gif、webp 图片")
	}

	nameExt := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if nameExt != "" && nameExt != ".jpeg" && nameExt != detectedExt {
		return "", "", errors.New("图片扩展名与内容不匹配")
	}

	return detectedExt, contentType, nil
}

// NewReadSeeker 将通用 reader 转为可复位 reader，用于 multipart.File 之外的测试。
func NewReadSeeker(data []byte) io.ReadSeeker {
	return bytes.NewReader(data)
}
