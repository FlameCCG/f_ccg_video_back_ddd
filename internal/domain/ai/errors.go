// Package ai 表达AI 参数校验、图片尺寸与站内提示词的纯规则。
//
// options.go/validation.go 校验模型选项，image_size.go 推导尺寸，prompt.go 组织已取得的作品事实。
// 本包不请求模型接口，不保证模型会遵从提示词；外部调用与返回值处理分属适配器和应用用例。
//
// 本包保持纯 Go 规则与端口，不依赖 HTTP、数据库、缓存、消息框架或配置加载器。
// 领域函数尽量显式接收事实与时间，让规则可以脱离运行环境阅读和验证。
package ai

import (
	"errors"
	"strconv"
)

// 领域错误。错误文案会经 handler 过滤后返回前端，属于外部可观测契约，不得修改。
var (
	// ErrConfigMissing AI 配置段缺失。
	ErrConfigMissing = errors.New("AI 配置未初始化")
	// ErrChatAPIKeyMissing 聊天模型 API Key 未配置。
	ErrChatAPIKeyMissing = errors.New("AI 聊天模型 API Key 未配置")
	// ErrDoubaoAPIKeyMissing 豆包模型 API Key 未配置。
	ErrDoubaoAPIKeyMissing = errors.New("豆包模型 API Key 未配置")
	// ErrEmptyInput 聊天入参 input 为空。
	ErrEmptyInput = errors.New("input 不能为空")
	// ErrNoUserMessage input 中没有可发送的用户消息。
	ErrNoUserMessage = errors.New("input 中没有可发送的用户消息")
	// ErrEmptyPrompt 图片生成 prompt 为空。
	ErrEmptyPrompt = errors.New("prompt 不能为空")
	// ErrImageCountRange 生成图片数量越界。
	ErrImageCountRange = errors.New("n 需在 1-15 之间")
	// ErrImageRefExclusive image 与 images 必须二选一。
	ErrImageRefExclusive = errors.New("image 和 images 必须二选一")
	// ErrTooManyImageRefs 图片编辑参考图数量超限。
	ErrTooManyImageRefs = errors.New("参考图片最多支持 14 张")
	// ErrImageRefPlusCount 参考图与生成图数量之和超限。
	ErrImageRefPlusCount = errors.New("参考图数量和生成图数量之和不能超过 15")
	// ErrTooManyVideoRefs 视频参考图数量超限。
	ErrTooManyVideoRefs = errors.New("reference_images 最多支持 7 张")
	// ErrVideoInputRequired 视频生成缺少任一输入。
	ErrVideoInputRequired = errors.New("prompt、image 或 reference_images 至少需要一个")
	// ErrVideoRefPromptRequired 参考图视频模式缺少 prompt。
	ErrVideoRefPromptRequired = errors.New("参考图视频模式要求 prompt 不能为空")
	// ErrVideoRefMixed 参考图不能与首帧图混用。
	ErrVideoRefMixed = errors.New("reference_images 不能与 image 或 input_reference 混用")
	// ErrEmptyRequestID 视频任务查询缺少 requestID。
	ErrEmptyRequestID = errors.New("requestID 不能为空")
	// ErrUnsupportedAssetScheme 资源代理仅支持 http/https。
	ErrUnsupportedAssetScheme = errors.New("仅支持 http/https 资源地址")
	// ErrAssetHostNotAllowed 资源地址不在代理白名单内。
	ErrAssetHostNotAllowed = errors.New("不支持代理该资源地址")
	// ErrOutputTextNotFound 上游响应中缺少 output_text。
	ErrOutputTextNotFound = errors.New("AI 分析响应中未找到 output_text")
)

// APIError 表示上游 AI 服务返回的业务错误.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Body       string
}

// Error 实现 error；文案格式属于外部可观测契约。
func (e *APIError) Error() string {
	if e == nil {
		return ""
	}

	if e.Code != "" {
		return "AI 请求失败[" + strconv.Itoa(e.StatusCode) + "/" + e.Code + "]: " + e.Message
	}

	return "AI 请求失败[" + strconv.Itoa(e.StatusCode) + "]: " + e.Message
}
