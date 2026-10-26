package config

const (
	testAdmin = "admin"
)

// AI 分别配置聊天、图片、视频和嵌入能力；各上游使用自己的模型标识与凭据。
// 豆包图/视频/向量 Embedding 走 Eino Ark SDK（默认方舟 BaseURL），无需配置 doubaoBaseURL / embeddingModelBaseURL.
// 聊天走 Eino Claude ChatModel 适配 Anthropic Messages 协议，
// 使用 chatModel + chatModelBaseURL + chatModelAPIKey，无全局 baseURL/apiKey/textModel 兜底.
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
	// MenuGenerationSystemPrompt 是管理员创建菜单时生成字段所使用的独立系统提示词。
	MenuGenerationSystemPrompt string    `mapstructure:"menuGenerationSystemPrompt" json:"menuGenerationSystemPrompt"`
	ImageModel                 string    `mapstructure:"imageModel" json:"imageModel"`
	ImageModels                []AIModel `mapstructure:"imageModels" json:"imageModels,omitempty"`
	VideoModel                 string    `mapstructure:"videoModel" json:"videoModel"`
	VideoModels                []AIModel `mapstructure:"videoModels" json:"videoModels,omitempty"`
	VectorIndex                string    `mapstructure:"vectorIndex" json:"vectorIndex"`
	// VectorTopK 向量召回 TopK；AI 找视频时第二轮注入候选作品数也按此上限。
	VectorTopK int `mapstructure:"vectorTopK" json:"vectorTopK"`
	// EmbeddingImageMode 控制封面图参与向量化：auto / on / off。
	EmbeddingImageMode string `mapstructure:"embeddingImageMode" json:"embeddingImageMode,omitempty"`
	// AssetBaseURL 把站内相对封面路径补为公网绝对地址。
	AssetBaseURL string `mapstructure:"assetBaseURL" json:"assetBaseURL,omitempty"`
	// VectorRRFRankConstant 是向量与关键词召回做 RRF 融合时的排名常数。
	VectorRRFRankConstant int `mapstructure:"vectorRRFRankConstant" json:"vectorRRFRankConstant,omitempty"`
	// VectorMinSimilarity 是向量召回最低余弦相似度；<=0 表示不过滤。
	VectorMinSimilarity float64 `mapstructure:"vectorMinSimilarity" json:"vectorMinSimilarity,omitempty"`
	// HistoryMaxTurns 是透传给聊天模型的最大历史消息条数。
	HistoryMaxTurns int `mapstructure:"historyMaxTurns" json:"historyMaxTurns,omitempty"`
	// HistoryMaxChars 是透传给聊天模型的历史文本总字符上限。
	HistoryMaxChars int `mapstructure:"historyMaxChars" json:"historyMaxChars,omitempty"`
	TimeoutSec      int `mapstructure:"timeoutSec" json:"timeoutSec"`
}

// AIModel 是一项可选配置的显示标签与实际值；界面展示 Label，调用上游使用 Value。
type AIModel struct {
	Label string `mapstructure:"label" json:"label"`
	Value string `mapstructure:"value" json:"value"`
}
