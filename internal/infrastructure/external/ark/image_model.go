package ark

import (
	"context"
	"fmt"
	"strings"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	domain "f_ccg_video_back_ddd/internal/domain/ai"
	"f_ccg_video_back_ddd/internal/infrastructure/config"

	arkmodel "github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/schema"
	arkrtmodel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"go.uber.org/zap"
)

// ImageModel 豆包文生图/图生图适配器（Eino 官方 ImageGenerationModel）。
type ImageModel struct {
	cfgProvider *config.Provider
}

var _ aiapp.ImageGenerator = (*ImageModel)(nil)

// NewImageModel 构造图片生成适配器。
func NewImageModel(cfgProvider *config.Provider) *ImageModel {
	return &ImageModel{cfgProvider: cfgProvider}
}

// Generate 通过 Eino 官方 ImageGenerationModel 调用豆包文生图/图生图。
func (m *ImageModel) Generate(ctx context.Context, inv aiapp.ImageInvocation) (any, error) {
	cfg, err := aiConfig(m.cfgProvider)
	if err != nil {
		return nil, err
	}

	size := domain.ResolveImageSize(inv.Model, inv.Size, inv.Resolution, inv.AspectRatio)
	responseFormat := domain.FirstNonEmpty(inv.ResponseFormat, "url")

	genConfig := &arkmodel.ImageGenerationConfig{
		APIKey:           doubaoAPIKey(cfg),
		Model:            inv.Model,
		Timeout:          arkTimeout(cfg),
		Size:             size,
		ResponseFormat:   arkmodel.ImageResponseFormat(responseFormat),
		DisableWatermark: true,
	}

	n := domain.RequestedImageCount(inv.N)
	if n > 1 {
		genConfig.SequentialImageGeneration = arkmodel.SequentialImageGenerationAuto
		genConfig.SequentialImageGenerationOption = &arkrtmodel.SequentialImageGenerationOptions{
			MaxImages: &n,
		}
	}

	model, err := arkmodel.NewImageGenerationModel(ctx, genConfig)
	if err != nil {
		return nil, fmt.Errorf("初始化豆包图片模型失败: %w", err)
	}

	msg, err := model.Generate(ctx, buildImageGenerationMessages(inv.Prompt, inv.Refs))
	if err != nil {
		zap.L().Error("generate AI image failed", zap.String("model", inv.Model), zap.Error(err))

		return nil, wrapArkError(err)
	}

	return imageMessageToAPIPayload(msg, inv.Model, responseFormat), nil
}

// buildImageGenerationMessages 组装 Eino 输入消息。
func buildImageGenerationMessages(prompt string, refs []string) []*schema.Message {
	urls := imageRefURLs(refs)
	text := strings.TrimSpace(prompt)

	// 纯文本走 Content；带参考图走 UserInputMultiContent（勿用已废弃的 MultiContent）。
	if len(urls) == 0 {
		return []*schema.Message{{
			Role:    schema.User,
			Content: text,
		}}
	}

	parts := make([]schema.MessageInputPart, 0, len(urls)+1)
	if text != "" {
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: text,
		})
	}

	for _, imageURL := range urls {
		url := imageURL
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					URL: &url,
				},
			},
		})
	}

	return []*schema.Message{{
		Role:                  schema.User,
		UserInputMultiContent: parts,
	}}
}

// imageRefURLs 过滤空白参考图地址。
func imageRefURLs(refs []string) []string {
	if len(refs) == 0 {
		return nil
	}

	out := make([]string, 0, len(refs))

	for _, item := range refs {
		if url := strings.TrimSpace(item); url != "" {
			out = append(out, url)
		}
	}

	return out
}

// imageMessageToAPIPayload 从 AssistantGenMultiContent 解析生成结果，组装上游兼容的 data 列表。
func imageMessageToAPIPayload(msg *schema.Message, model, responseFormat string) map[string]any {
	items := make([]any, 0)

	if msg != nil {
		for _, part := range msg.AssistantGenMultiContent {
			if part.Image == nil {
				continue
			}

			item := map[string]any{}

			switch {
			case part.Image.URL != nil && strings.TrimSpace(*part.Image.URL) != "":
				url := strings.TrimSpace(*part.Image.URL)
				if responseFormat == "b64_json" {
					item["b64_json"] = url
				} else {
					item["url"] = url
				}
			case part.Image.Base64Data != nil && strings.TrimSpace(*part.Image.Base64Data) != "":
				item["b64_json"] = strings.TrimSpace(*part.Image.Base64Data)
			default:
				continue
			}

			items = append(items, item)
		}
	}

	return map[string]any{
		"model": model,
		"data":  items,
	}
}
