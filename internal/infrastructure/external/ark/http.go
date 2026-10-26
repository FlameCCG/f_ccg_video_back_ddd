package ark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	domain "f_ccg_video_back_ddd/internal/domain/ai"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	arkrtmodel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// errorBodyLimit 读取上游错误响应体的上限。
const errorBodyLimit = 4096

// httpClientFor 流式请求不设整体超时（由 context 控制），非流式取配置与 AI 基线的较大值。
func httpClientFor(runtime aiapp.RuntimeConfig, stream bool) *http.Client {
	if stream {
		return &http.Client{}
	}

	timeout := ctxutil.RequestAITimeout
	if runtime.TimeoutSec > 0 {
		timeout = max(time.Duration(runtime.TimeoutSec)*time.Second, ctxutil.RequestAITimeout)
	}

	return &http.Client{Timeout: timeout}
}

// gatedStreamHTTPClient 阻止流式响应体在 Eino Stream 返回前被消费。
//
// Eino Claude 组件会先启动读取 goroutine，再完成 Stream 的返回值装配；如果上游首包
// 极快，组件内部命名返回值会被两个 goroutine 同时写。适配器在 Stream 返回后打开闸门，
// 既不缓存响应体，也不改变后续流式读取和取消语义。
func gatedStreamHTTPClient() (*http.Client, func()) {
	ready := make(chan struct{})
	var once sync.Once
	release := func() {
		once.Do(func() {
			close(ready)
		})
	}

	return &http.Client{
		Transport: &responseReadGateTransport{
			base:  http.DefaultTransport,
			ready: ready,
		},
	}, release
}

type responseReadGateTransport struct {
	base  http.RoundTripper
	ready <-chan struct{}
}

func (t *responseReadGateTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	// SDK 会在 Stream 返回前同步读取非 2xx 错误体；给它加闸门会互相等待。
	if response.Body == nil || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return response, nil
	}

	response.Body = &responseReadGate{
		ReadCloser: response.Body,
		ctx:        request.Context(),
		ready:      t.ready,
	}

	return response, nil
}

type responseReadGate struct {
	io.ReadCloser
	ctx   context.Context
	ready <-chan struct{}
}

func (g *responseReadGate) Read(buffer []byte) (int, error) {
	select {
	case <-g.ready:
		return g.ReadCloser.Read(buffer)
	case <-g.ctx.Done():
		return 0, g.ctx.Err()
	}
}

// parseAPIError 把上游错误响应解析为领域 APIError。
func parseAPIError(statusCode int, body []byte) error {
	apiErr := &domain.APIError{
		StatusCode: statusCode,
		Body:       strings.TrimSpace(string(body)),
	}

	var payload struct {
		Message string `json:"message"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &payload); err == nil {
		apiErr.Code = strings.TrimSpace(payload.Error.Code)
		apiErr.Message = domain.FirstNonEmpty(
			strings.TrimSpace(payload.Error.Message),
			strings.TrimSpace(payload.Message),
		)
	}

	if apiErr.Message == "" {
		apiErr.Message = fmt.Sprintf("状态码=%d", statusCode)
	}

	return apiErr
}

// wrapArkError 把方舟 SDK 错误映射为领域 APIError。
func wrapArkError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *arkrtmodel.APIError
	if errors.As(err, &apiErr) && apiErr != nil {
		return &domain.APIError{
			StatusCode: apiErr.HTTPStatusCode,
			Code:       apiErr.Code,
			Message:    apiErr.Message,
			Body:       err.Error(),
		}
	}

	var reqErr *arkrtmodel.RequestError
	if errors.As(err, &reqErr) && reqErr != nil {
		return &domain.APIError{
			StatusCode: reqErr.HTTPStatusCode,
			Message:    reqErr.Error(),
			Body:       reqErr.Error(),
		}
	}

	return err
}
