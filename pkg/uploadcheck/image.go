// Package uploadcheck 在图片进入存储前校验大小和内容类型。
//
// image.go 读取前 512 字节探测 MIME 类型，文件扩展名或请求 Content-Type 不能独立证明文件有效。
// 本包不写对象存储，校验后复位 reader，调用方再将同一数据流和检测类型交给上传适配器。
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
	// DefaultMaxImageBytes 是未指定上限时使用的 10 MiB 图片大小限制，单位为字节。
	DefaultMaxImageBytes = 10 * 1024 * 1024
)

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

	detectedExt, ok := imageExtensionForMIME(contentType)
	if !ok {
		return "", "", errors.New("仅支持 jpg、png、gif、webp 图片")
	}

	nameExt := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if nameExt != "" && nameExt != ".jpeg" && nameExt != detectedExt {
		return "", "", errors.New("图片扩展名与内容不匹配")
	}

	return detectedExt, contentType, nil
}

func imageExtensionForMIME(contentType string) (string, bool) {
	switch contentType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/gif":
		return ".gif", true
	case "image/webp":
		return ".webp", true
	default:
		return "", false
	}
}

// NewReadSeeker 将通用 reader 转为可复位 reader，用于 multipart.File 之外的测试。
func NewReadSeeker(data []byte) io.ReadSeeker {
	return bytes.NewReader(data)
}
