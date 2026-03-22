package ai

import (
	"strings"
)

const (
	// DefaultCandidateLimit 站内作品候选默认上限。
	DefaultCandidateLimit = 8
	// MaxVideoResultLinkCount 一轮回答中最多返回的站内链接数。
	MaxVideoResultLinkCount = 5
	// MaxVideoQueryKeywords 第一轮意图分析最多提炼的关键词数。
	MaxVideoQueryKeywords = 6
	// AnalysisMaxOutputTokens 第一轮意图分析的最大输出 token 数。
	AnalysisMaxOutputTokens = 256
	// DefaultAnalysisTimeoutSec 第一轮意图分析的默认超时秒数。
	DefaultAnalysisTimeoutSec = 20
	// maxKeywordRunes 单个关键词最大字符数（超过视为噪声丢弃）。
	maxKeywordRunes = 32
	// maxDescriptionRunes 候选作品描述截断长度。
	maxDescriptionRunes = 240
)

// 候选来源（写入提示词，影响模型的引用口径）。
const (
	// RetrievalModeVector 语义向量召回。
	RetrievalModeVector = "vector_match"
	// RetrievalModeImageVector 用户图片参与的多模态向量召回。
	RetrievalModeImageVector = "image_vector_match"
	// RetrievalModeKeyword 关键词命中。
	RetrievalModeKeyword = "keyword_match"
	// RetrievalModeLatest 关键词未命中后的最新发布回退。
	RetrievalModeLatest = "latest_fallback"
)

// VideoQueryAnalysisPrompt 第一轮「是否在找站内视频」的意图分析提示词。
const VideoQueryAnalysisPrompt = `你是站内视频检索意图分析器。
请严格输出 JSON 对象，格式只能是：{"intent":"find_video","keywords":["关键词1","关键词2"]}。

规则：
1. intent 只能是 find_video 或 chat。
2. 如果当前对话是在找站内视频、作品、分P、求推荐、问有没有某类内容，或者想根据主题/人物/技术名词/上传的图片找站内作品，intent 取 find_video，并返回 1 到 6 个适合数据库模糊匹配的关键词或短语。
3. 关键词要尽量短、尽量准，优先保留专有名词、英文单词、版本号、技术词、人物名、作品名、tag 词。
4. 不要保留“帮我、找一下、推荐一下、有没有、视频、作品、站内、请”这类无意义词。
5. 如果用户上传了图片并想找相似/同款/同一作品的站内视频，intent 取 find_video；关键词可以为空数组，图片会由后端单独做向量检索。
6. 如果这轮是普通聊天、解释问题、单纯让你描述图片、对上一条结果的追问、寒暄、感谢、闲聊，intent 取 chat，keywords 返回空数组。
7. 不要输出解释、不要输出 markdown、不要输出代码块，只输出符合 schema 的 JSON。`

// VideoQueryAnalysis 第一轮意图分析的结构化结果。
type VideoQueryAnalysis struct {
	// Intent 为 find_video 时才做召回；兼容旧格式：Intent 为空但有关键词也视为找视频。
	Intent   string
	Keywords []string
}

// WantsVideoSearch 判断意图分析结果是否要求执行站内作品召回。
func (analysis VideoQueryAnalysis) WantsVideoSearch() bool {
	switch strings.ToLower(strings.TrimSpace(analysis.Intent)) {
	case "find_video":
		return true
	case "chat":
		return false
	default:
		return len(analysis.Keywords) > 0
	}
}

// PromptVideoPart 候选作品的分 P 业务摘要。
type PromptVideoPart struct {
	P     int
	Title string
}

// PromptVideoCandidate 经业务规则整理的站内作品候选，不包含展示路由或序列化格式。
type PromptVideoCandidate struct {
	VideoID     uint
	Title       string
	Description string
	Partition   string
	Cover       string
	Tags        []string

	WorkType      string
	PartSummaries []PromptVideoPart
}

// PromptVideoCandidateSet 一轮召回的候选集合与来源。
type PromptVideoCandidateSet struct {
	RetrievalMode string
	QueryKeywords []string
	UsedImage     bool
	Candidates    []PromptVideoCandidate
}

// VideoPartInput 构造候选所需的分 P 原始数据（由 infrastructure 从存储映射而来）。
type VideoPartInput struct {
	Title string
}

// VideoCandidateInput 构造候选所需的作品原始数据（由 infrastructure 从存储映射而来）。
type VideoCandidateInput struct {
	VideoID     uint
	Title       string
	Description string
	Partition   string
	Cover       string
	Tags        []string
	Parts       []VideoPartInput
}

// NewPromptVideoCandidate 由作品原始数据构造提示词候选项。
//
// 规则：描述截断 240 字，
// 分 P 多于一个时标记为 multi_part 并保留分 P 摘要。
func NewPromptVideoCandidate(input VideoCandidateInput) PromptVideoCandidate {
	item := PromptVideoCandidate{
		VideoID:     input.VideoID,
		Title:       strings.TrimSpace(input.Title),
		Description: (PromptVideoCandidate{}).TrimPromptText(input.Description, maxDescriptionRunes),
		Partition:   strings.TrimSpace(input.Partition),
		Cover:       strings.TrimSpace(input.Cover),

		WorkType: "single",
		Tags:     input.normalizeTagNames(),
	}

	if len(input.Parts) > 1 {
		item.WorkType = "multi_part"

		item.PartSummaries = make([]PromptVideoPart, 0, len(input.Parts))

		for idx, part := range input.Parts {
			p := idx + 1
			item.PartSummaries = append(item.PartSummaries, PromptVideoPart{
				P:     p,
				Title: strings.TrimSpace(part.Title),
			})
		}
	}

	return item
}

// RetrievalGuidance 按召回来源选择引用和推荐口径。
func (set PromptVideoCandidateSet) RetrievalGuidance() string {
	switch set.RetrievalMode {
	case RetrievalModeImageVector:
		return "- 当前候选来源：用户上传图片的多模态向量召回结果（图片与作品封面、标题、标签在同一向量空间比对）。请优先从这些作品中选择与图片最相关的内容，并简要说明为什么像；如果都不像，就直说没找到相似作品。\n"
	case RetrievalModeVector:
		return "- 当前候选来源：语义向量召回结果，请优先从这些作品中选择最相关的内容。\n"
	case RetrievalModeKeyword:
		return "- 当前候选来源：关键词命中结果，请优先从这些作品中选择最相关的内容。\n"
	case RetrievalModeLatest:
		return "- 当前候选来源：检索未命中任何已发布作品，后端已按最新发布时间回退提供候选作品。你需要自行判断是否值得推荐；如果都不合适，就明确说暂时没有特别匹配的站内结果。\n"
	default:
		return "- 当前候选来源：没有候选作品。\n"
	}
}

// NormalizeAnalysisKeywords 规范化意图分析关键词：去空、去超长、忽略大小写去重、截断到上限。
func (VideoQueryAnalysis) NormalizeAnalysisKeywords(keywords []string) []string {
	if len(keywords) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(keywords))

	out := make([]string, 0, len(keywords))

	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}

		if len([]rune(keyword)) > maxKeywordRunes {
			continue
		}

		normalizedKey := strings.ToLower(keyword)
		if _, ok := seen[normalizedKey]; ok {
			continue
		}

		seen[normalizedKey] = struct{}{}

		out = append(out, keyword)
		if len(out) >= MaxVideoQueryKeywords {
			break
		}
	}

	return out
}

// EnsurePositiveLimit 保证候选上限为正数，否则回落默认值。
func (PromptVideoCandidateSet) EnsurePositiveLimit(limit int) int {
	if limit > 0 {
		return limit
	}

	return DefaultCandidateLimit
}

// FirstNonEmpty 返回第一个非空（去空格后）字符串，用于提示词与同类文本回落。
func (PromptVideoCandidate) FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

// TrimPromptText 按字符数截断文本，超长时追加省略号。
func (PromptVideoCandidate) TrimPromptText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" || maxRunes <= 0 {
		return value
	}

	if len([]rune(value)) <= maxRunes {
		return value
	}

	var builder strings.Builder

	count := 0

	for _, r := range value {
		builder.WriteRune(r)

		count++
		if count >= maxRunes {
			break
		}
	}

	return builder.String() + "..."
}

// normalizeTagNames 过滤空白 tag 名；无有效值时返回 nil。
func (input VideoCandidateInput) normalizeTagNames() []string {
	if len(input.Tags) == 0 {
		return nil
	}

	names := make([]string, 0, len(input.Tags))

	for _, tag := range input.Tags {
		name := strings.TrimSpace(tag)
		if name != "" {
			names = append(names, name)
		}
	}

	return names
}
