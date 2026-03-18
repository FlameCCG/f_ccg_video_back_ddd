// Package encrypt 封装 bcrypt 密码散列与验证。
package encrypt

import (
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
