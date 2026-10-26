package identity

import (
	"context"
	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/pkg/apperr"
	"strconv"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/identity"

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

// ParseRefresh 解析 refresh 令牌并返回其中的用户 ID；非法时返回 ErrRefreshTokenInvalid。
func (s *TokenService) ParseRefresh(refreshToken string) (uint, error) {
	jwtConf := s.jwtConf()
	if jwtConf == nil {
		return 0, ErrRefreshTokenInvalid
	}

	uid, err := jwts.ParseRefreshToken(refreshToken, jwtConf.RefreshSecret)
	if err != nil || uid == "" {
		return 0, apperr.Wrap("application/identity/token_service", ErrRefreshTokenInvalid, err, apperr.Rejected)
	}

	uidInt, err := strconv.Atoi(uid)
	if err != nil || uidInt <= 0 {
		return 0, apperr.Wrap("application/identity/token_service", ErrRefreshTokenInvalid, err, apperr.Rejected)
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

// IssueForUser persists the refresh digest; a subsequent login replaces the prior
// refresh session while existing access tokens remain bounded by credential state.
func (s *TokenService) IssueForUser(ctx context.Context, user *domain.User, users domain.UserAccountRepository) (*jwts.JwtToken, error) {
	pair, err := s.issueCredentialPair(user)
	if err != nil {
		return nil, err
	}

	ok, err := users.SetRefreshToken(ctx, user.ID, user.Password, jwts.TokenDigest(pair.RefreshToken))
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, ErrRefreshTokenInvalid
	}

	return pair, nil
}

func (s *TokenService) issueCredentialPair(user *domain.User) (*jwts.JwtToken, error) {
	conf := s.jwtConf()
	if conf == nil {
		return nil, ErrRefreshTokenInvalid
	}

	return jwts.CreateJwtTokenWithPassword(strconv.FormatUint(uint64(user.ID), 10), user.Username, user.Password, user.MustChangePassword, configuredDuration(conf.AccessExp, 24*time.Hour), conf.AccessSecret, configuredDuration(conf.RefreshExp, 24*time.Hour), conf.RefreshSecret)
}

// Rotate atomically replaces the stored refresh digest and issues a new token pair.
func (s *TokenService) Rotate(ctx context.Context, user *domain.User, oldToken string, users domain.UserAccountRepository) (*jwts.JwtToken, error) {
	conf := s.jwtConf()
	if conf == nil {
		return nil, ErrRefreshTokenInvalid
	}

	if err := jwts.CheckPasswordVersion(oldToken, conf.RefreshSecret, user.Password, "refresh"); err != nil {
		return nil, apperr.Wrap("application/identity/token_service", ErrRefreshTokenInvalid, err, apperr.Rejected)
	}

	pair, err := s.issueCredentialPair(user)
	if err != nil {
		return nil, err
	}

	ok, err := users.RotateRefreshToken(ctx, user.ID, user.Password, jwts.TokenDigest(oldToken), jwts.TokenDigest(pair.RefreshToken))
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, ErrRefreshTokenReused
	}

	return pair, nil
}
