// Package encrypt 封装密码散列与文件摘要计算。
//
// 密码使用 bcrypt；MD5/SHA-256 工具用于摘要、标识或完整性比较，两类用途不能混用。
// FileHashEqualsData 比较文件和字节数据的 SHA-256 及大小；HashFileSHA256 用流式读取避免整文件入内存。
package encrypt

import (
	"crypto/md5" //nolint:gosec // Legacy non-authentication digest format must remain compatible with stored identifiers.
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// GenPasswordHash 用 bcrypt 默认成本为密码生成带盐散列；相同密码每次得到的散列可不同。
func GenPasswordHash(password []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
}

// ValidatePasswordHash 比较明文密码与 bcrypt 散列，校验失败或散列格式错误都返回 false。
func ValidatePasswordHash(password, hashed string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)); err != nil {
		return false
	}

	return true
}

// Md5 返回字节数据的小写十六进制 MD5 摘要，用于标识/比较，不用于保存密码。
func Md5(data []byte) string {
	//nolint:gosec // Compatibility checksum only; authentication and passwords use stronger primitives.
	md5New := md5.New()
	md5New.Write(data)
	md5Str := hex.EncodeToString(md5New.Sum(nil))

	return md5Str
}

// Md5Str 读取整个文件并计算 MD5，读取失败返回错误；大文件处理需注意其整文件内存占用。
func Md5Str(filepath string) (string, error) {
	bytesData, err := os.ReadFile(filepath) // #nosec G304 -- This file utility intentionally reads an explicit caller-selected path; callers enforce their storage boundary.
	if err != nil {
		return "", err
	}

	md5Str := Md5(bytesData)

	return md5Str, nil
}

// FileHashEqualsData 同时比较文件和给定数据的 SHA-256 与长度；读文件失败视为不相等。
func FileHashEqualsData(path string, data []byte) bool {
	hashExisting, size, err := HashFileSHA256(path)
	if err != nil {
		return false
	}

	hashIncoming := HashBytesSHA256(data)

	return strings.EqualFold(hashExisting, hashIncoming) && size == int64(len(data))
}

// HashFileSHA256 流式计算文件 SHA-256 并返回 Stat 得到的字节数；任何打开或读取错误向上传递。
func HashFileSHA256(path string) (hash string, size int64, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}

	size = fi.Size()

	f, err := os.Open(path) // #nosec G304 -- This file utility intentionally reads an explicit caller-selected path; callers enforce their storage boundary.
	if err != nil {
		return "", 0, err
	}

	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", 0, err
	}

	return hex.EncodeToString(h.Sum(nil)), size, nil
}

// HashBytesSHA256 计算内存字节数据的 SHA-256，返回小写十六进制字符串。
func HashBytesSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
