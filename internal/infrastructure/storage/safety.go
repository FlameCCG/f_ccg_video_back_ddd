package storage

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var safeUploadIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// ValidateUploadID 校验用户上传目录标识，避免路径穿越和异常对象 key。
func ValidateUploadID(uploadID string) error {
	uploadID = strings.TrimSpace(uploadID)
	if uploadID == "" {
		return errors.New("fileHash 不能为空")
	}

	if !safeUploadIDPattern.MatchString(uploadID) || strings.Contains(uploadID, "..") {
		return errors.New("fileHash 格式无效")
	}

	return nil
}

// ValidateChunkIndex 校验分片索引，分片文件名只能是非负十进制整数。
func ValidateChunkIndex(index string) error {
	index = strings.TrimSpace(index)
	if index == "" {
		return errors.New("分片索引不能为空")
	}

	n, err := strconv.Atoi(index)
	if err != nil || n < 0 || strconv.Itoa(n) != index {
		return errors.New("分片索引格式无效")
	}

	return nil
}

// cleanObjectKey 归一化对象存储 key，并拒绝绝对路径与目录穿越。
func cleanObjectKey(relativePath string) (string, error) {
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return "", errors.New("object key 不能为空")
	}

	if strings.ContainsAny(relativePath, "\\\x00") || path.IsAbs(relativePath) {
		return "", errors.New("object key 格式无效")
	}

	key := path.Clean(relativePath)
	if key == "." || key == ".." || strings.HasPrefix(key, "../") {
		return "", errors.New("object key 格式无效")
	}

	return key, nil
}

// safeJoinLocal 把相对路径安全地拼接到 base 目录内，拒绝任何越界结果。
func safeJoinLocal(base string, elems ...string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", errors.New("base path 不能为空")
	}

	baseAbs, err := filepath.Abs(filepath.Clean(base))
	if err != nil {
		return "", err
	}

	relative := filepath.Join(elems...)
	if strings.TrimSpace(relative) == "" || strings.ContainsRune(relative, '\x00') || filepath.IsAbs(relative) {
		return "", errors.New("relative path 格式无效")
	}

	cleaned := filepath.Clean(relative)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", errors.New("relative path 格式无效")
	}

	fullPath, err := filepath.Abs(filepath.Join(baseAbs, cleaned))
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(baseAbs, fullPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("relative path %q outside of base dir", relative)
	}

	return fullPath, nil
}
