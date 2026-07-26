// Package jwts provides package functionality.
package jwts

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// JwtToken 类型.
type JwtToken struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	AccessExp    int64  `json:"accessExp"`
	RefreshExp   int64  `json:"refreshExp"`
}

// CreateJwtToken 签发 access/refresh token；secret 为空或签名失败时返回错误。
func CreateJwtToken(uid, username string, exp time.Duration, secret string, refreshExp time.Duration, refreshSecret string) (*JwtToken, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("jwt access secret is required")
	}
	if strings.TrimSpace(refreshSecret) == "" {
		return nil, errors.New("jwt refresh secret is required")
	}

	aExp := time.Now().Add(exp).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid":      uid,
		"username": username,
		"exp":      aExp,
	})
	aToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	rExp := time.Now().Add(refreshExp).Unix()
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid": uid,
		"exp": rExp,
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

// ParseJwtToken 导出符号.
func ParseJwtToken(tokenString, secret string) (uid, username string, err error) {
	uid, username, err = ParseJwtTokenWithRole(tokenString, secret)
	return uid, username, err
}

// ParseJwtTokenWithRole 导出符号.
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

// ParseRefreshToken 导出符号.
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
