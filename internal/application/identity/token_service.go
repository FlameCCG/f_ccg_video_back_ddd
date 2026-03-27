package identity

import (
	"context"
	"strconv"
	"time"

	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/config"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"github.com/google/uuid"
)

// TokenService 负责签发与解析业务令牌。
//
// JWT 携带 uid、sid 与密码版本；有效期配置单位为天，登录和刷新响应保持令牌对结构。
type TokenService struct {
	cfgProvider shared.ConfigReader
	sessions    SessionStore
}

// NewTokenService 构造令牌服务。
func NewTokenService(cfgProvider shared.ConfigReader, sessions SessionStore) *TokenService {
	return &TokenService{cfgProvider: cfgProvider, sessions: sessions}
}

// ParseRefresh 解析 refresh 令牌并返回其中的用户 ID；非法时返回 ErrRefreshTokenInvalid。
func (s *TokenService) ParseRefresh(refreshToken string) (uint, error) {
	jwtConf := s.jwtConf()
	if jwtConf == nil {
		return 0, ErrRefreshTokenInvalid
	}

	claims, err := jwts.ParseRefreshClaims(refreshToken, jwtConf.RefreshSecret)
	if err != nil {
		return 0, ErrRefreshTokenInvalid
	}

	uidInt, err := strconv.ParseUint(claims.UID, 10, strconv.IntSize)
	if err != nil || uidInt == 0 {
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

// IssueForUser 为每次登录建立独立会话，已有设备的刷新凭据不受影响。
func (s *TokenService) IssueForUser(ctx context.Context, user *identitydomain.User) (*jwts.JwtToken, error) {
	sessionID := uuid.NewString()
	pair, err := s.issueCredentialPair(user, sessionID)
	if err != nil {
		return nil, err
	}
	if s.sessions == nil {
		return nil, ErrRefreshTokenInvalid
	}
	if err := s.sessions.Create(ctx, user.ID(), sessionID, jwts.TokenDigest(pair.RefreshToken), time.Unix(pair.RefreshExp, 0)); err != nil {
		return nil, err
	}
	return pair, nil
}
func (s *TokenService) issueCredentialPair(user *identitydomain.User, sessionID string) (*jwts.JwtToken, error) {
	if user == nil || user.ID() == 0 {
		return nil, ErrRefreshTokenInvalid
	}
	conf := s.jwtConf()
	if conf == nil {
		return nil, ErrRefreshTokenInvalid
	}
	return jwts.CreateJwtTokenWithPassword(strconv.FormatUint(uint64(user.ID()), 10), user.Username(), user.Password(), sessionID, conf.AccessExp*24*time.Hour, conf.AccessSecret, conf.RefreshExp*24*time.Hour, conf.RefreshSecret)
}
func (s *TokenService) Rotate(ctx context.Context, user *identitydomain.User, oldToken string) (*jwts.JwtToken, error) {
	conf := s.jwtConf()
	if conf == nil {
		return nil, ErrRefreshTokenInvalid
	}
	claims, err := jwts.ParseRefreshClaims(oldToken, conf.RefreshSecret)
	if err != nil || claims.UID != strconv.FormatUint(uint64(user.ID()), 10) || s.sessions == nil {
		return nil, ErrRefreshTokenInvalid
	}
	if err := claims.CheckPasswordVersion(conf.RefreshSecret, user.Password()); err != nil {
		return nil, ErrRefreshTokenInvalid
	}
	pair, err := s.issueCredentialPair(user, claims.SessionID)
	if err != nil {
		return nil, err
	}
	ok, err := s.sessions.Rotate(ctx, user.ID(), claims.SessionID, jwts.TokenDigest(oldToken), jwts.TokenDigest(pair.RefreshToken), time.Unix(pair.RefreshExp, 0))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrRefreshTokenInvalid
	}
	return pair, nil
}
