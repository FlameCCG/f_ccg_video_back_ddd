package ai

import (
	"fmt"
	"strings"
)

// 视频模型标识（时长限制按模型区分）。
const (
	proVideoModel  = "doubao-seedance-1-5-pro-251215"
	fastVideoModel = "doubao-seedance-1-0-pro-fast-251015"
)

const (
	// maxImageCount 单次生成图片数量上限。
	maxImageCount = 15
	// maxImageEditRefs 图片编辑参考图数量上限。
	maxImageEditRefs = 14
	// maxVideoRefs 视频参考图数量上限。
	maxVideoRefs = 7
)

// ValidateImageGeneration 校验文生图请求（prompt 必填、n 越界）。
func ValidateImageGeneration(prompt string, n *int) error {
	if strings.TrimSpace(prompt) == "" {
		return ErrEmptyPrompt
	}

	if n != nil && (*n <= 0 || *n > maxImageCount) {
		return ErrImageCountRange
	}

	return nil
}

// ValidateImageEdit 校验图生图/图片编辑请求。
//
// 判定顺序与原 prepareImageEditRequest 完全一致：
// prompt → image/images 互斥 → 参考图上限 → n 越界 → 参考图与生成图之和。
func ValidateImageEdit(prompt string, hasImage bool, imagesLen int, n *int) error {
	if strings.TrimSpace(prompt) == "" {
		return ErrEmptyPrompt
	}

	hasImages := imagesLen > 0
	if hasImage == hasImages {
		return ErrImageRefExclusive
	}

	refCount := imagesLen
	if hasImage {
		refCount = 1
	}

	if refCount > maxImageEditRefs {
		return ErrTooManyImageRefs
	}

	count := 1
	if n != nil {
		count = *n
	}

	if count <= 0 || count > maxImageCount {
		return ErrImageCountRange
	}

	if refCount+count > maxImageCount {
		return ErrImageRefPlusCount
	}

	return nil
}

// ValidateVideoGeneration 校验视频生成请求的输入组合。
//
// 判定顺序与原 prepareVideoGenerationRequest 完全一致：
// 参考图上限 → 至少一个输入 → 参考图模式必须带 prompt → 参考图与首帧图不可混用。
func ValidateVideoGeneration(prompt string, hasImage bool, refCount int) error {
	if refCount > maxVideoRefs {
		return ErrTooManyVideoRefs
	}

	if !hasImage && refCount == 0 && strings.TrimSpace(prompt) == "" {
		return ErrVideoInputRequired
	}

	if refCount > 0 && strings.TrimSpace(prompt) == "" {
		return ErrVideoRefPromptRequired
	}

	if hasImage && refCount > 0 {
		return ErrVideoRefMixed
	}

	return nil
}

// ResolveDuration 从 duration / seconds 解析视频时长；均缺省时返回 (0, false)。
func ResolveDuration(duration, seconds *int) (int, bool) {
	if duration != nil {
		return *duration, true
	}

	if seconds != nil {
		return *seconds, true
	}

	return 0, false
}

// VideoDurationLimits 返回指定视频模型允许的时长区间（秒）。
func VideoDurationLimits(model string) (int, int) {
	switch model {
	case proVideoModel:
		return 4, 15
	case fastVideoModel:
		return 2, 12
	default:
		return 1, 15
	}
}

// ValidateVideoDuration 校验视频时长是否在模型允许区间内。
func ValidateVideoDuration(model string, duration int) error {
	minDuration, maxDuration := VideoDurationLimits(model)
	if duration < minDuration || duration > maxDuration {
		return fmt.Errorf("duration/seconds 需在 %d-%d 秒之间", minDuration, maxDuration)
	}

	return nil
}

// NormalizeVideoResolution 规范化视频分辨率；非法值返回空串表示不下发该参数。
func NormalizeVideoResolution(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "480p", "720p", "1080p":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

// NormalizeVideoRatio 规范化画面比例；非法值返回空串表示不下发该参数。
func NormalizeVideoRatio(value string) string {
	value = strings.TrimSpace(value)
	switch value {
	case "adaptive", test169, "4:3", "1:1", "3:4", "9:16", "21:9":
		return value
	default:
		return ""
	}
}

// IsAllowedAssetHost 判断资源域名是否在代理白名单内（安全边界，防 SSRF）。
func IsAllowedAssetHost(rawHost string) bool {
	host := strings.ToLower(strings.TrimSpace(rawHost))
	for _, suffix := range []string{
		".volces.com",
		".volcengine.com",
		".byteimg.com",
		".bytedance.com",
		".tos-cn-beijing.volces.com",
	} {
		if strings.HasSuffix(host, suffix) || strings.TrimPrefix(suffix, ".") == host {
			return true
		}
	}

	return false
}
