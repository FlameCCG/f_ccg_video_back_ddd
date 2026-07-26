package ark

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	domain "f_ccg_video_back_ddd/internal/domain/ai"
	"f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	arkrtmodel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// VideoTaskClient 豆包视频生成任务适配器（方舟官方 runtime SDK，Eino 暂无视频组件）。
type VideoTaskClient struct {
	cfgProvider *config.Provider
}

var _ aiapp.VideoTaskClient = (*VideoTaskClient)(nil)

// NewVideoTaskClient 构造视频生成任务适配器。
func NewVideoTaskClient(cfgProvider *config.Provider) *VideoTaskClient {
	return &VideoTaskClient{cfgProvider: cfgProvider}
}

// Create 创建上游视频生成任务。
func (c *VideoTaskClient) Create(ctx context.Context, inv aiapp.VideoInvocation) (any, error) {
	cfg, err := aiConfig(c.cfgProvider)
	if err != nil {
		return nil, err
	}

	content := make([]*arkrtmodel.CreateContentGenerationContentItem, 0, 1+len(inv.ImageRefs))
	if prompt := strings.TrimSpace(inv.Prompt); prompt != "" {
		text := prompt
		content = append(content, &arkrtmodel.CreateContentGenerationContentItem{
			Type: arkrtmodel.ContentGenerationContentItemTypeText,
			Text: &text,
		})
	}

	for _, ref := range inv.ImageRefs {
		url := strings.TrimSpace(ref)
		if url == "" {
			continue
		}

		content = append(content, &arkrtmodel.CreateContentGenerationContentItem{
			Type: arkrtmodel.ContentGenerationContentItemTypeImage,
			ImageURL: &arkrtmodel.ImageURL{
				URL: url,
			},
		})
	}

	payload := arkrtmodel.CreateContentGenerationTaskRequest{
		Model:   inv.Model,
		Content: content,
	}

	if resolution := domain.NormalizeVideoResolution(inv.Resolution); resolution != "" {
		payload.Resolution = &resolution
	}

	if ratio := domain.NormalizeVideoRatio(domain.FirstNonEmpty(inv.AspectRatio, inv.Size)); ratio != "" {
		payload.Ratio = &ratio
	}

	if duration, ok := domain.ResolveDuration(inv.Duration, inv.Seconds); ok {
		d := int64(duration)
		payload.Duration = &d
	}

	if inv.GenerateAudio != nil {
		payload.GenerateAudio = inv.GenerateAudio
	}

	if inv.CameraFixed != nil {
		payload.CameraFixed = inv.CameraFixed
	}

	if inv.Watermark != nil {
		payload.Watermark = inv.Watermark
	}

	if inv.Seed != nil {
		seed := int64(*inv.Seed)
		payload.Seed = &seed
	}

	resp, err := newArkRuntimeClient(cfg).CreateContentGenerationTask(ctx, payload)
	if err != nil {
		return nil, wrapArkError(err)
	}

	return normalizeVideoTaskResponse(resp)
}

// Get 查询上游视频生成任务状态。
func (c *VideoTaskClient) Get(ctx context.Context, requestID string) (any, error) {
	cfg, err := aiConfig(c.cfgProvider)
	if err != nil {
		return nil, err
	}

	resp, err := newArkRuntimeClient(cfg).GetContentGenerationTask(ctx, arkrtmodel.GetContentGenerationTaskRequest{
		ID: requestID,
	})
	if err != nil {
		return nil, wrapArkError(err)
	}

	return normalizeVideoTaskResponse(resp)
}

// newArkRuntimeClient 构建方舟官方 runtime 客户端（Eino Ark 组件底层同款，BaseURL 走 SDK 默认）。
func newArkRuntimeClient(cfg *config.AI) *arkruntime.Client {
	opts := []arkruntime.ConfigOption{}
	if cfg != nil && cfg.TimeoutSec > 0 {
		opts = append(opts, arkruntime.WithTimeout(time.Duration(cfg.TimeoutSec)*time.Second))
	}

	return arkruntime.NewClientWithApiKey(doubaoAPIKey(cfg), opts...)
}

// normalizeVideoTaskResponse 序列化 SDK 响应后补齐 requestId 字段。
func normalizeVideoTaskResponse(resp any) (any, error) {
	raw, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("序列化视频任务响应失败: %w", err)
	}

	return decodeAndNormalizeTask(raw)
}

// decodeAndNormalizeTask 解析任务响应并补齐 requestId（前端按该字段轮询）。
func decodeAndNormalizeTask(raw []byte) (any, error) {
	data, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}

	root, ok := data.(map[string]any)
	if !ok {
		return data, nil
	}

	if _, exists := root["requestId"]; exists {
		return root, nil
	}

	if id := findTaskID(root); id != "" {
		root["requestId"] = id
	}

	return root, nil
}

// findTaskID 在任务响应中递归寻找任务 ID。
func findTaskID(node any) string {
	switch value := node.(type) {
	case map[string]any:
		for _, key := range []string{"id", "task_id", "taskId", "request_id", "requestId"} {
			if raw, ok := value[key].(string); ok && strings.TrimSpace(raw) != "" {
				return strings.TrimSpace(raw)
			}
		}

		for _, item := range value {
			if id := findTaskID(item); id != "" {
				return id
			}
		}
	case []any:
		for _, item := range value {
			if id := findTaskID(item); id != "" {
				return id
			}
		}
	}

	return ""
}

func decodeJSON(raw []byte) (any, error) {
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("解析 AI 响应失败: %w", err)
	}

	return data, nil
}
