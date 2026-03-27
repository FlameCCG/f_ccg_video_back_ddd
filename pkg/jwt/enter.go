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
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	AccessExp    int64  `json:"accessExp"`
	RefreshExp   int64  `json:"refreshExp"`
}

// AccessClaims contains claims from a verified access token. Credential checks
// reuse these claims without parsing or verifying the signature again.
type AccessClaims struct {
	UID       string
	Username  string
	SessionID string
	version   string
}

func ParseAccessToken(tokenString, secret string) (*AccessClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	uid, err := claimString(claims, "uid")
	if err != nil {
		return nil, err
	}
	username, err := claimString(claims, "username")
	if err != nil {
		return nil, err
	}
	exp, err := claimUnix(claims, "exp")
	if err != nil || exp <= time.Now().Unix() {
		return nil, errors.New("token expired or missing expiry")
	}
	if use, ok := claims["tokenUse"].(string); !ok || use != "access" {
		return nil, errors.New("incorrect token use")
	}
	if jti, ok := claims["jti"].(string); !ok || jti == "" {
		return nil, errors.New("invalid token")
	}
	version, ok := claims["pwdVer"].(string)
	if !ok || version == "" {
		return nil, errors.New("invalid credential version")
	}
	sessionID, err := claimString(claims, "sid")
	if err != nil {
		return nil, err
	}
	return &AccessClaims{UID: uid, Username: username, SessionID: sessionID, version: version}, nil
}

func (c *AccessClaims) CheckPasswordVersion(secret, passwordHash string) error {
	if c == nil || !hmac.Equal([]byte(c.version), []byte(PasswordVersion(passwordHash, secret))) {
		return errors.New("credentials changed")
	}
	return nil
}

// RefreshClaims 只携带通过刷新令牌签名、用途与有效期检查后的会话身份。
type RefreshClaims struct {
	UID       string
	SessionID string
	version   string
}

func (c *RefreshClaims) CheckPasswordVersion(secret, passwordHash string) error {
	if c == nil || !hmac.Equal([]byte(c.version), []byte(PasswordVersion(passwordHash, secret))) {
		return errors.New("credentials changed")
	}
	return nil
}

func ParseRefreshClaims(tokenString, secret string) (*RefreshClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	uid, err := claimString(claims, "uid")
	if err != nil {
		return nil, err
	}
	sessionID, err := claimString(claims, "sid")
	if err != nil {
		return nil, err
	}
	exp, err := claimUnix(claims, "exp")
	if err != nil || exp <= time.Now().Unix() {
		return nil, errors.New("token expired or missing expiry")
	}
	if use, ok := claims["tokenUse"].(string); !ok || use != "refresh" {
		return nil, errors.New("incorrect token use")
	}
	version, err := claimString(claims, "pwdVer")
	if err != nil {
		return nil, err
	}
	if _, err := claimString(claims, "jti"); err != nil {
		return nil, err
	}
	return &RefreshClaims{UID: uid, SessionID: sessionID, version: version}, nil
}

// CreateJwtTokenWithPassword binds both tokens to the current credential state.
func CreateJwtTokenWithPassword(uid, username, passwordHash, sessionID string, exp time.Duration, secret string, refreshExp time.Duration, refreshSecret string) (*JwtToken, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("jwt session id is required")
	}
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
	tokenID := hex.EncodeToString(entropy[:])
	aExp := time.Now().Add(exp).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid":      uid,
		"username": username,
		"pwdVer":   PasswordVersion(passwordHash, secret),
		"tokenUse": "access",
		"sid":      sessionID,
		"jti":      tokenID,
		"exp":      aExp,
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
		"sid":      sessionID,
		"jti":      tokenID,
		"exp":      rExp,
	})
	rToken, err := refreshToken.SignedString([]byte(refreshSecret))
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	return &JwtToken{
		AccessToken:  aToken,
		AccessExp:    aExp,
		RefreshToken: rToken,
		RefreshExp:   rExp,
	}, nil
}

// ParseRefreshToken 验证刷新令牌的签名、用途、会话与有效期，并提取用户 ID。
func ParseRefreshToken(tokenString, secret string) (string, error) {
	claims, err := ParseRefreshClaims(tokenString, secret)
	if err != nil {
		return "", err
	}
	return claims.UID, nil
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
func TokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
