// Package oauth 实现第三方 OAuth 登录与一次性 state 存储的外部协议适配。
//
// 各 provider 文件处理上游令牌和资料接口，state_store.go 以 Redis 消费 state，retry.go 约束 HTTP 重试。
// state 的一次性校验防止回调串用；本包取得外部身份事实后，由 identity 用例决定本地账号与令牌。
//
// 上层依赖端口而非客户端库，测试可替换端口或 HTTP 服务以验证协议边界。
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
