package identity

import (
	"strconv"
	"time"

	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/config"
	jwts "f_ccg_video_back_ddd/pkg/jwt"
)

// TokenService 负责签发与解析业务令牌。
//
// JWT claims（uid/username/exp）、有效期换算（配置单位为天）与 cookie/响应字段
// 全部沿用原 api/user_api 各登录接口的写法，属于外部契约。
type TokenService struct {
	cfgProvider shared.ConfigReader
}

// NewTokenService 构造令牌服务。
func NewTokenService(cfgProvider shared.ConfigReader) *TokenService {
	return &TokenService{cfgProvider: cfgProvider}
}

// Issue 为指定用户签发 access/refresh 令牌对。
func (s *TokenService) Issue(userID uint, username string) (*jwts.JwtToken, error) {
	jwtConf := s.jwtConf()
	if jwtConf == nil {
		jwtConf = &config.Jwt{}
	}

	return jwts.CreateJwtToken(
		strconv.Itoa(int(userID)),
		username,
		jwtConf.AccessExp*24*time.Hour,
		jwtConf.AccessSecret,
		jwtConf.RefreshExp*24*time.Hour,
		jwtConf.RefreshSecret,
	)
}

// ParseRefresh 解析 refresh 令牌并返回其中的用户 ID；非法时返回 ErrRefreshTokenInvalid。
func (s *TokenService) ParseRefresh(refreshToken string) (uint, error) {
	jwtConf := s.jwtConf()
	if jwtConf == nil {
		return 0, ErrRefreshTokenInvalid
	}

	uid, err := jwts.ParseRefreshToken(refreshToken, jwtConf.RefreshSecret)
	if err != nil || uid == "" {
		return 0, ErrRefreshTokenInvalid
	}

	uidInt, err := strconv.Atoi(uid)
	if err != nil || uidInt <= 0 {
		return 0, ErrRefreshTokenInvalid
	}

	return uint(uidInt), nil
}

func (s *TokenService) jwtConf() *config.Jwt {
	cfg := s.cfgProvider.Get()
	if cfg == nil {
		return nil
	}

	return cfg.Jwt
}
