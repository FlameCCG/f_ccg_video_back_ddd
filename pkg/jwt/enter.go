// Package jwts 签发和解析访问令牌与刷新令牌。
//
// 访问与刷新令牌使用不同密钥和解析入口；本包校验签名、声明与过期时间，不直接读取角色权限。
// 本包依赖显式配置，HTTP 中间件把验证后的声明转换为请求身份；数据库封禁和路由授权仍是后续检查。
package jwts

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// JwtToken 把访问/刷新令牌及各自的 Unix 秒级过期时间作为一个登录结果返回。
type JwtToken struct {
	MustChangePassword bool   `json:"mustChangePassword,omitempty"`
	AccessToken        string `json:"accessToken"`
	RefreshToken       string `json:"refreshToken"`
	AccessExp          int64  `json:"accessExp"`
	RefreshExp         int64  `json:"refreshExp"`
}

// CreateJwtToken 签发 access/refresh token；secret 为空或签名失败时返回错误。
func CreateJwtToken(uid, username string, exp time.Duration, secret string, refreshExp time.Duration, refreshSecret string) (*JwtToken, error) {
	return CreateJwtTokenWithPassword(uid, username, "", false, exp, secret, refreshExp, refreshSecret)
}

// CreateJwtTokenWithPassword binds both tokens to the current credential state.
func CreateJwtTokenWithPassword(uid, username, passwordHash string, mustChange bool, exp time.Duration, secret string, refreshExp time.Duration, refreshSecret string) (*JwtToken, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("jwt access secret is required")
	}

	if strings.TrimSpace(refreshSecret) == "" {
		return nil, errors.New("jwt refresh secret is required")
	}

	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return nil, err
	}

	sessionID := hex.EncodeToString(entropy[:])
	aExp := time.Now().Add(exp).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid":                uid,
		"username":           username,
		"pwdVer":             PasswordVersion(passwordHash, secret),
		"tokenUse":           "access",
		"mustChangePassword": mustChange,
		"jti":                sessionID,
		"exp":                aExp,
	})

	aToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	rExp := time.Now().Add(refreshExp).Unix()
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid":      uid,
		"tokenUse": "refresh",
		"pwdVer":   PasswordVersion(passwordHash, refreshSecret),
		"jti":      sessionID,
		"exp":      rExp,
	})

	rToken, err := refreshToken.SignedString([]byte(refreshSecret))
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	return &JwtToken{
		MustChangePassword: mustChange,
		AccessToken:        aToken,
		AccessExp:          aExp,
		RefreshToken:       rToken,
		RefreshExp:         rExp,
	}, nil
}

// ParseJwtToken 复用访问令牌解析逻辑，返回用户 ID 与用户名；不查询数据库或角色策略。
func ParseJwtToken(tokenString, secret string) (uid, username string, err error) {
	uid, username, err = ParseJwtTokenWithRole(tokenString, secret)
	return uid, username, err
}

// ParseJwtTokenWithRole 验证 HMAC 签名、声明形状与过期时间后返回 uid/username。
// 名称保留历史叫法，当前实现不返回角色；角色权限仍需经服务端 Casbin 策略判断。
func ParseJwtTokenWithRole(tokenString, secret string) (uid, username string, err error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})
	if err != nil {
		return uid, username, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		uid, err = claimString(claims, "uid")
		if err != nil {
			return uid, username, err
		}

		username, err = claimString(claims, "username")
		if err != nil {
			return uid, username, err
		}

		var exp int64

		exp, err = claimUnix(claims, "exp")
		if err != nil {
			return "", "", err
		}

		if exp <= time.Now().Unix() {
			err = errors.New("token expired")
			return uid, username, err
		}

		return uid, username, err
	}

	err = errors.New("invalid token")

	return uid, username, err
}

// ParseRefreshToken 使用调用方给定的刷新密钥验证令牌并提取 uid/exp，不要求 username 声明。
// 访问与刷新令牌的隔离依赖应用层正确选择解析入口及各自密钥。
func ParseRefreshToken(tokenString, secret string) (uid string, err error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})
	if err != nil {
		return uid, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		uid, err = claimString(claims, "uid")
		if err != nil {
			return uid, err
		}

		var exp int64

		exp, err = claimUnix(claims, "exp")
		if err != nil {
			return "", err
		}

		if exp <= time.Now().Unix() {
			err = errors.New("token expired")
			return uid, err
		}

		return uid, err
	}

	err = errors.New("invalid token")

	return uid, err
}

func claimString(claims jwt.MapClaims, key string) (string, error) {
	switch v := claims[key].(type) {
	case string:
		if v == "" {
			return "", errors.New("invalid token")
		}

		return v, nil
	case float64:
		return strconv.FormatInt(int64(v), 10), nil
	default:
		return "", errors.New("invalid token")
	}
}

func claimUnix(claims jwt.MapClaims, key string) (int64, error) {
	switch v := claims[key].(type) {
	case float64:
		return int64(v), nil
	case json.Number:
		return v.Int64()
	case string:
		exp, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, errors.New("invalid token")
		}

		return exp, nil
	default:
		return 0, errors.New("invalid token")
	}
}

// PasswordVersion is a one-way, server-keyed credential version. Password changes
// change the bcrypt hash (including its random salt), invalidating all old tokens.
func PasswordVersion(passwordHash, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("jwt-credential-version:" + passwordHash))

	return hex.EncodeToString(mac.Sum(nil))
}

// CheckPasswordVersion verifies a signed token against current credentials.
func CheckPasswordVersion(tokenString, secret, passwordHash string, expectedUse ...string) error {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}

		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return errors.New("invalid token")
	}

	if len(expectedUse) > 0 {
		if use, ok := claims["tokenUse"].(string); !ok || use != expectedUse[0] {
			return errors.New("incorrect token use")
		}
	}

	version, ok := claims["pwdVer"].(string)
	if !ok || !hmac.Equal([]byte(version), []byte(PasswordVersion(passwordHash, secret))) {
		return errors.New("credentials changed")
	}

	if jti, ok := claims["jti"].(string); !ok || jti == "" {
		return errors.New("invalid token")
	}

	return nil
}

// TokenDigest returns the SHA256 digest used to persist refresh credentials.
func TokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
