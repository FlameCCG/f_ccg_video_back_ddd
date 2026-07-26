// Package ai 是 AI 上下文的应用层：请求 DTO、上游端口与用例服务。
package ai

import (
	"encoding/json"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/ai"
)

// ResponsesRequest 表示聊天/文本生成请求.
//
// 该类型同时是路由层泛型绑定的目标（binding tag 与原 ai_service 逐字一致）。
type ResponsesRequest struct {
	Model string          `json:"model,omitempty"`
	Input json.RawMessage `json:"input" binding:"required"`
	// Thinking 为本次请求是否开启思考；nil 时跟随后台 thinkingEnabled.
	// 仅当后台 thinkingEnabled=true 时，客户端可将此设为 false 以关闭思考.
	Thinking *bool `json:"thinking,omitempty"`
	// ThinkingEffort 思考强度（如 high / max）；空则用后台默认.
	ThinkingEffort string `json:"thinkingEffort,omitempty"`
}

// ImageGenerationRequest 表示文生图请求参数.
type ImageGenerationRequest struct {
	Model          string `json:"model,omitempty"`
	Prompt         string `json:"prompt" binding:"required"`
	N              *int   `json:"n,omitempty"`
	AspectRatio    string `json:"aspect_ratio,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	Quality        string `json:"quality,omitempty"`
	User           string `json:"user,omitempty"`
	Size           string `json:"size,omitempty"`
	Style          string `json:"style,omitempty"`
}

// ImageEditRequest 表示图生图/图片编辑请求参数.
type ImageEditRequest struct {
	Model          string     `json:"model,omitempty"`
	Prompt         string     `json:"prompt" binding:"required"`
	Image          *ImageRef  `json:"image,omitempty"`
	Images         []ImageRef `json:"images,omitempty"`
	Mask           string     `json:"mask,omitempty"`
	N              *int       `json:"n,omitempty"`
	AspectRatio    string     `json:"aspect_ratio,omitempty"`
	Resolution     string     `json:"resolution,omitempty"`
	ResponseFormat string     `json:"response_format,omitempty"`
	Quality        string     `json:"quality,omitempty"`
	User           string     `json:"user,omitempty"`
	Size           string     `json:"size,omitempty"`
	Style          string     `json:"style,omitempty"`
}

// VideoGenerationRequest 表示视频生成请求参数.
type VideoGenerationRequest struct {
	Model           string          `json:"model,omitempty"`
	Prompt          string          `json:"prompt,omitempty"`
	Duration        *int            `json:"duration,omitempty"`
	Seconds         *int            `json:"seconds,omitempty"`
	AspectRatio     string          `json:"aspect_ratio,omitempty"`
	Resolution      string          `json:"resolution,omitempty"`
	Image           *ImageRef       `json:"image,omitempty"`
	InputReference  *ImageRef       `json:"input_reference,omitempty"`
	ReferenceImages []ImageRef      `json:"reference_images,omitempty"`
	GenerateAudio   *bool           `json:"generate_audio,omitempty"`
	CameraFixed     *bool           `json:"camera_fixed,omitempty"`
	Watermark       *bool           `json:"watermark,omitempty"`
	Seed            *int            `json:"seed,omitempty"`
	Output          json.RawMessage `json:"output,omitempty"`
	User            string          `json:"user,omitempty"`
	Size            string          `json:"size,omitempty"`
}

// ImageRef 表示图片引用（URL 或类型化对象）.
type ImageRef struct {
	URL  string `json:"url,omitempty"`
	Type string `json:"type,omitempty"`
}

// UnmarshalJSON 支持字符串 URL 与对象两种图片引用格式.
func (ref *ImageRef) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*ref = ImageRef{}

		return nil
	}

	var rawString string
	if err := json.Unmarshal(data, &rawString); err == nil {
		ref.URL = strings.TrimSpace(rawString)
		ref.Type = domain.ContentTypeImageURL

		return nil
	}

	type alias ImageRef

	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	ref.URL = strings.TrimSpace(parsed.URL)

	ref.Type = strings.TrimSpace(parsed.Type)
	if ref.URL != "" && ref.Type == "" {
		ref.Type = domain.ContentTypeImageURL
	}

	return nil
}

// MarshalJSON 序列化图片引用，缺省 type 为 image_url.
func (ref ImageRef) MarshalJSON() ([]byte, error) {
	type alias ImageRef

	payload := alias{
		URL:  strings.TrimSpace(ref.URL),
		Type: strings.TrimSpace(ref.Type),
	}
	if payload.Type == "" {
		payload.Type = domain.ContentTypeImageURL
	}

	return json.Marshal(payload)
}

// IsZero 判断图片引用是否为空.
func (ref ImageRef) IsZero() bool {
	return strings.TrimSpace(ref.URL) == ""
}

// PublicOptions 前台 AI 模型与思考配置（不含密钥）.
//
// json tag 与原 ai_service.PublicOptions 逐字一致；选项列表元素为 domain.Option，
// 其 label/value tag 与原 conf.AIModel 完全相同。
type PublicOptions struct {
	ChatModel       string          `json:"chatModel"`
	ChatModels      []domain.Option `json:"chatModels"`
	ThinkingEnabled bool            `json:"thinkingEnabled"`
	ThinkingEffort  string          `json:"thinkingEffort"`
	ThinkingEfforts []domain.Option `json:"thinkingEfforts"`
	ImageModel      string          `json:"imageModel"`
	ImageModels     []domain.Option `json:"imageModels"`
	VideoModel      string          `json:"videoModel"`
	VideoModels     []domain.Option `json:"videoModels"`
}
