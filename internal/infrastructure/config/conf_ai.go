package config

// AI 类型.
// 豆包图/视频/向量 Embedding 走 Eino Ark SDK（默认方舟 BaseURL），无需配置 doubaoBaseURL / embeddingModelBaseURL.
// 聊天走 chatModel + chatModelBaseURL + chatModelAPIKey，无全局 baseURL/apiKey/textModel 兜底.
type AI struct {
	ChatModelBaseURL string    `mapstructure:"chatModelBaseURL" json:"chatModelBaseURL"`
	ChatModelAPIKey  string    `mapstructure:"chatModelAPIKey" json:"chatModelAPIKey"`
	ChatModel        string    `mapstructure:"chatModel" json:"chatModel"`
	ChatModels       []AIModel `mapstructure:"chatModels" json:"chatModels,omitempty"`
	// ThinkingEnabled 为 true 时，前台可开启思考并选择思考强度；为 false 时强制关闭思考.
	ThinkingEnabled bool `mapstructure:"thinkingEnabled" json:"thinkingEnabled"`
	// ThinkingEffort 默认思考强度（如 high / max）.
	ThinkingEffort string `mapstructure:"thinkingEffort" json:"thinkingEffort"`
	// ThinkingEfforts 可选思考强度列表（管理端/前台下拉）.
	ThinkingEfforts     []AIModel `mapstructure:"thinkingEfforts" json:"thinkingEfforts,omitempty"`
	EmbeddingModel      string    `mapstructure:"embeddingModel" json:"embeddingModel"`
	EmbeddingDimensions int       `mapstructure:"embeddingDimensions" json:"embeddingDimensions"`
	DoubaoModelAPIKey   string    `mapstructure:"doubaoModelAPIKey" json:"doubaoModelAPIKey"`
	SystemPrompt        string    `mapstructure:"systemPrompt" json:"systemPrompt"`
	ImageModel          string    `mapstructure:"imageModel" json:"imageModel"`
	ImageModels         []AIModel `mapstructure:"imageModels" json:"imageModels,omitempty"`
	VideoModel          string    `mapstructure:"videoModel" json:"videoModel"`
	VideoModels         []AIModel `mapstructure:"videoModels" json:"videoModels,omitempty"`
	VectorIndex         string    `mapstructure:"vectorIndex" json:"vectorIndex"`
	// VectorTopK 向量召回 TopK；AI 找视频时第二轮注入候选作品数也按此上限。
	VectorTopK int `mapstructure:"vectorTopK" json:"vectorTopK"`
	TimeoutSec int `mapstructure:"timeoutSec" json:"timeoutSec"`
}

// AIModel 类型.
type AIModel struct {
	Label string `mapstructure:"label" json:"label"`
	Value string `mapstructure:"value" json:"value"`
}
