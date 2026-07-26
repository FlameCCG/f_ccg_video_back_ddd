// Package oauth 提供第三方登录（QQ / Google / GitHub / Linux.do / X）的外部调用适配器，
// 以及一次性 OAuth state 存储。对应原 service/login_service 及其子包。
//
// 本包实现 application/identity 中声明的 OAuthStateStore 与各 provider 客户端端口；
// 授权/令牌/用户信息端点、请求参数、超时与重试策略均与原实现逐字一致。
package oauth

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

// oauthRetryBackoff 重试前的固定退避时长。
const oauthRetryBackoff = 250 * time.Millisecond

// DoWithRetry 对 OAuth 相关 HTTP 请求做有限次重试（最多 2 次尝试）。
//
// 仅在网络超时、5xx 或 429 时重试；重试前会读完并关闭上一次响应体。
func DoWithRetry(
	client *http.Client,
	buildRequest func() (*http.Request, error),
) (*http.Response, error) {
	var lastErr error

	for attempt := range 2 {
		req, err := buildRequest()
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if !shouldRetryOAuth(attempt, err, resp) {
			return resp, err
		}

		lastErr = err

		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}

		time.Sleep(oauthRetryBackoff)
	}

	return nil, lastErr
}

// CloneRequestBody 读取并缓存请求体，便于重试时复用。
func CloneRequestBody(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	return io.ReadAll(body)
}

// NewBodyReader 根据缓存字节构造可重复读取的请求体。
func NewBodyReader(body []byte) io.Reader {
	if len(body) == 0 {
		return nil
	}

	return bytes.NewReader(body)
}

func shouldRetryOAuth(attempt int, err error, resp *http.Response) bool {
	if attempt >= 1 {
		return false
	}

	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return true
		}

		return false
	}

	if resp == nil {
		return false
	}

	return resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests
}
