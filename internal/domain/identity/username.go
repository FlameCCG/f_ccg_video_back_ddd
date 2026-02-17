package identity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UsernamePrefix 自动生成用户名的固定前缀。
const UsernamePrefix = "毛民"

// UniqueShortUUID 生成「UUID 前 8 位 + 4 位纳秒时间戳」的短标识。
func UniqueShortUUID() string {
	u := uuid.New()
	timestamp := time.Now().UnixNano() % 10000

	return fmt.Sprintf("%s%04d", u.String()[:8], timestamp)
}

// GenerateUsername 为第三方首次登录用户生成默认用户名。
func GenerateUsername() string {
	return UsernamePrefix + "_" + UniqueShortUUID()
}
