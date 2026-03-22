package identity

import (
	"fmt"
	"time"
)

// UsernamePrefix 自动生成用户名的固定前缀。
const UsernamePrefix = "毛民"

// UsernamePolicy 为第三方首次登录生成默认用户名。
type UsernamePolicy struct{}

// Generate 生成「前缀 + 短标识」用户名。
func (policy UsernamePolicy) Generate(id string, now time.Time) string {
	return UsernamePrefix + "_" + policy.shortToken(id, now)
}

// shortToken 生成「UUID 前 8 位 + 4 位纳秒时间戳」的短标识。
func (UsernamePolicy) shortToken(id string, now time.Time) string {
	timestamp := now.UnixNano() % 10000

	return fmt.Sprintf("%s%04d", id[:8], timestamp)
}
