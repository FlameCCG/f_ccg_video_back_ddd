package identity

import (
	"strings"
	"time"
)

// OAuthStateTTL 一次性 OAuth state 的有效期（与原 login_service.oauthStateTTL 一致）。
const OAuthStateTTL = 10 * time.Minute

// NormalizeOAuthState 去除 provider/state 首尾空白并校验非空。
func NormalizeOAuthState(provider, state string) (string, string, error) {
	provider = strings.TrimSpace(provider)
	state = strings.TrimSpace(state)

	if provider == "" || state == "" {
		return "", "", ErrOAuthStateInvalid
	}

	return provider, state, nil
}

// OAuthStateKey 拼装 state 存储键（Redis key 格式属于外部契约，不可更改）。
func OAuthStateKey(provider, state string) string {
	return "oauth:state:" + provider + ":" + state
}
