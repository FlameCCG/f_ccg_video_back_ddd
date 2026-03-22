package identity

import (
	"strings"
	"time"
)

// OAuthStateTTL 一次性 OAuth state 的有效期。
const OAuthStateTTL = 10 * time.Minute

// OAuthStatePolicy 规范化并校验一次性 OAuth state。
// state 本身仍是公开字符串，不封装成实体。
type OAuthStatePolicy struct{}

// Normalize 去除 provider/state 首尾空白并校验非空。
func (OAuthStatePolicy) Normalize(provider, state string) (string, string, error) {
	provider = strings.TrimSpace(provider)
	state = strings.TrimSpace(state)

	if provider == "" || state == "" {
		return "", "", ErrOAuthStateInvalid
	}

	return provider, state, nil
}
