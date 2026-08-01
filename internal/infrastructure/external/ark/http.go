package ark

import (
	"bytes"
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

// doProviderJSONRequest 发送 JSON 请求并读取完整响应体，非 2xx 转成 APIError。
func doProviderJSONRequest(
	ctx context.Context,
	runtime aiapp.RuntimeConfig,
	baseURL string,
	apiKey string,
	method string,
	path string,
	payload any,
	accept string,
	headers map[string]string,
) (json.RawMessage, error) {
	resp, err := doProviderRequest(ctx, runtime, baseURL, apiKey, method, path, payload, accept, headers, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 AI 响应失败: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, parseAPIError(resp.StatusCode, body)
	}

	return body, nil
}

// openProviderJSONStream 打开 SSE 流；非 2xx 时读干错误体并转成 APIError。
func openProviderJSONStream(
	ctx context.Context,
	runtime aiapp.RuntimeConfig,
	baseURL string,
	apiKey string,
	method string,
	path string,
	payload any,
	headers map[string]string,
) (*http.Response, error) {
	resp, err := doProviderRequest(
		ctx,
		runtime,
		baseURL,
		apiKey,
		method,
		path,
		payload,
		"text/event-stream",
		headers,
		true,
	)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if readErr != nil {
			return nil, fmt.Errorf("读取 AI 流式错误响应失败: %w", readErr)
		}

		return nil, parseAPIError(resp.StatusCode, body)
	}

	return resp, nil
}

// doProviderRequest 构建并发送上游请求。
func doProviderRequest(
	ctx context.Context,
	runtime aiapp.RuntimeConfig,
	baseURL string,
	apiKey string,
	method string,
	path string,
	payload any,
	accept string,
	headers map[string]string,
	stream bool,
) (*http.Response, error) {
	var body io.Reader

	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("序列化 AI 请求失败: %w", err)
		}

		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, domain.JoinURL(baseURL, path), body)
	if err != nil {
		return nil, fmt.Errorf("构建 AI 请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	req.Header.Set("Accept", domain.FirstNonEmpty(accept, "application/json"))

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}

	resp, err := httpClientFor(runtime, stream).Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 AI 失败: %w", err)
	}

	return resp, nil
}

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
	if response.Body == nil {
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
