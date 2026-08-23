package ai

import (
	"context"
	"net/http"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/ai"

	"github.com/cloudwego/eino/schema"
)

// RuntimeConfig 是一次 AI 公开用例调用捕获的上游运行配置。
//
// 它属于应用层端口契约，不暴露配置机制细节。值在用例入口完成
// 规范化后按值传递，因此配置热更新不会让同一次调用混用不同版本的地址、密钥或超时。
type RuntimeConfig struct {
	ChatModelBaseURL string
	ChatModelAPIKey  string
	DoubaoAPIKey     string
	TimeoutSec       int
}

// TextInvocation 聊天模型调用参数（已完成模型/思考选项校验与系统提示词注入）。
type TextInvocation struct {
	Runtime RuntimeConfig
	Model   string
	// Messages 是解析、裁剪并注入服务端系统提示词后的 Eino 标准消息。
	Messages []*schema.Message
	// Thinking 是否开启思考；nil 视为关闭。
	Thinking *bool
	// ThinkingEffort 思考强度，空表示不下发。
	ThinkingEffort string
	// MaxTokens 最大输出 token 数。
	MaxTokens int
}

// ImageInvocation 图片生成调用参数。
type ImageInvocation struct {
	Runtime        RuntimeConfig
	Model          string
	Prompt         string
	N              *int
	AspectRatio    string
	Resolution     string
	ResponseFormat string
	Size           string
	// Refs 参考图地址列表；为空表示纯文生图。
	Refs []string
}

// VideoInvocation 视频生成任务参数。
type VideoInvocation struct {
	Runtime       RuntimeConfig
	Model         string
	Prompt        string
	Duration      *int
	Seconds       *int
	AspectRatio   string
	Resolution    string
	Size          string
	ImageRefs     []string
	GenerateAudio *bool
	CameraFixed   *bool
	Watermark     *bool
	Seed          *int
}

// TextGenerator 聊天文本生成端口（上游为 Anthropic 兼容接口）。
//
// 实现见 infrastructure/external/ark：负责上游请求体拼装、SSE 事件翻译与错误映射。
type TextGenerator interface {
	// InvokeText 非流式生成。
	InvokeText(ctx context.Context, inv TextInvocation) (string, error)
	// OpenStream 打开 Eino 消息流；调用方负责 Close。
	OpenStream(ctx context.Context, inv TextInvocation) (*schema.StreamReader[*schema.Message], error)
}

// ImageGenerator 文生图/图生图端口（上游为方舟 Ark 图片模型）。
type ImageGenerator interface {
	// Generate 生成图片并返回上游兼容的响应载荷。
	Generate(ctx context.Context, inv ImageInvocation) (any, error)
}

// VideoTaskClient 视频生成任务端口（上游为方舟 runtime SDK）。
type VideoTaskClient interface {
	// Create 创建视频生成任务。
	Create(ctx context.Context, inv VideoInvocation) (any, error)
	// Get 查询视频生成任务状态。
	Get(ctx context.Context, runtime RuntimeConfig, requestID string) (any, error)
}

// AssetFetcher 上游资源代理拉取端口。
//
// 调用方（应用服务）已完成 scheme 与域名白名单校验，适配器只做转发。
type AssetFetcher interface {
	// Open 拉取资源流；返回值的 Body 由调用方负责关闭。
	Open(
		ctx context.Context,
		runtime RuntimeConfig,
		assetURL string,
		incomingHeaders http.Header,
	) (*http.Response, error)
}

// PromptVideoQuery 站内作品候选读侧投影端口。
//
// 原实现在 ai_service/video_prompt.go 直接查 videos / video_parts / tags 表；
// 按 D-011，读侧投影允许直读其它上下文的表，实现放在本上下文的
// infrastructure/persistence/ai（视频写侧仍归 video 上下文）。
type PromptVideoQuery interface {
	// Available 报告底层数据库是否可用；不可用时应用服务跳过候选注入。
	Available() bool
	// KeywordSearchIDs 关键词命中已发布作品 ID（按匹配优先级 + 发布时间排序）。
	KeywordSearchIDs(ctx context.Context, keywords []string, limit int) ([]uint, error)
	// LatestPublishedIDs 最新发布作品 ID（关键词未命中时的回退）。
	LatestPublishedIDs(ctx context.Context, limit int) ([]uint, error)
	// Load 按给定 ID 顺序加载候选作品原始数据（缺失的 ID 直接跳过）。
	Load(ctx context.Context, ids []uint) ([]domain.VideoCandidateInput, error)
}

// VideoVectorSearch 站内视频向量召回端口（跨上下文）。
//
// 由 search 上下文的应用服务在 bootstrap 中绑定实现
// （原 search_service.SearchVideoVectorIDs）。
type VideoVectorSearch interface {
	// SearchVideoVectorIDsMultiModal 按文本、图片或二者组合召回已发布作品 ID。
	SearchVideoVectorIDsMultiModal(ctx context.Context, query VideoVectorQuery, limit int) ([]uint, error)
}

// VideoVectorQuery 是 AI 上下文传给搜索上下文的多模态查询值对象。
type VideoVectorQuery struct {
	Text     string
	ImageURL string
}

// IsZero 判断查询是否同时缺少文本与图片。
func (query VideoVectorQuery) IsZero() bool {
	return strings.TrimSpace(query.Text) == "" && strings.TrimSpace(query.ImageURL) == ""
}
