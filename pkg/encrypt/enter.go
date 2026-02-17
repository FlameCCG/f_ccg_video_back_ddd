// Package encrypt provides package functionality.
package encrypt

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// GenPasswordHash 导出符号.
func GenPasswordHash(password []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
}

// ValidatePasswordHash 导出符号.
func ValidatePasswordHash(password, hashed string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)); err != nil {
		return false
	}

	return true
}

// Md5 导出符号.
func Md5(data []byte) string {
	md5New := md5.New()
	md5New.Write(data)
	md5Str := hex.EncodeToString(md5New.Sum(nil))

	return md5Str
}

// Md5Str 导出符号.
func Md5Str(filepath string) (string, error) {
	bytesData, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}

	md5Str := Md5(bytesData)

	return md5Str, nil
}

// FileHashEqualsData 导出符号.
func FileHashEqualsData(path string, data []byte) bool {
	hashExisting, size, err := HashFileSHA256(path)
	if err != nil {
		return false
	}

	hashIncoming := HashBytesSHA256(data)

	return strings.EqualFold(hashExisting, hashIncoming) && size == int64(len(data))
}

// HashFileSHA256 导出符号.
func HashFileSHA256(path string) (hash string, size int64, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}

	size = fi.Size()

	f, err := os.Open(path)
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

// HashBytesSHA256 导出符号.
func HashBytesSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
