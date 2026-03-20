package ai

import (
	"encoding/json"
	"strings"

	aivo "f_ccg_video_back_ddd/internal/domain/ai/valueobject"
)

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
		ref.Type = aivo.ContentTypeImageURL

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
		ref.Type = aivo.ContentTypeImageURL
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
		payload.Type = aivo.ContentTypeImageURL
	}

	return json.Marshal(payload)
}

// IsZero 判断图片引用是否为空.
func (ref ImageRef) IsZero() bool {
	return strings.TrimSpace(ref.URL) == ""
}
