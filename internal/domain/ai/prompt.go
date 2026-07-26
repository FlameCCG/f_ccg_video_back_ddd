package ai

import (
	"encoding/json"
	"fmt"
	"strconv"
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
	AnalysisMaxOutputTokens = 96
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
	// RetrievalModeKeyword 关键词命中。
	RetrievalModeKeyword = "keyword_match"
	// RetrievalModeLatest 关键词未命中后的最新发布回退。
	RetrievalModeLatest = "latest_fallback"
)

// VideoQueryAnalysisPrompt 第一轮「是否在找站内视频」的意图分析提示词。
const VideoQueryAnalysisPrompt = `你是站内视频检索意图分析器。
请严格输出 JSON 对象，格式只能是：{"keywords":["关键词1","关键词2"]}。

规则：
1. 如果当前对话是在找站内视频、作品、分P、求推荐、问有没有某类内容，或者想根据主题/人物/技术名词找站内作品，请返回 1 到 6 个适合数据库模糊匹配的关键词或短语。
2. 关键词要尽量短、尽量准，优先保留专有名词、英文单词、版本号、技术词、人物名、作品名、tag 词。
3. 不要保留“帮我、找一下、推荐一下、有没有、视频、作品、站内、请”这类无意义词。
4. 如果这轮不是在找站内视频，而是普通聊天、解释问题、图片理解、对上一条结果的追问、寒暄、感谢、闲聊，返回空数组。
5. 不要输出解释、不要输出 markdown、不要输出代码块，只输出符合 schema 的 JSON。`

// VideoQueryAnalysis 第一轮意图分析的结构化结果。
type VideoQueryAnalysis struct {
	Keywords []string `json:"keywords"`
}

// PromptVideoPart 候选作品的分 P 摘要（json tag 参与提示词内容，属于外部契约）.
type PromptVideoPart struct {
	P     int    `json:"p"`
	Title string `json:"title"`
	Route string `json:"route"`
}

// PromptVideoCandidate 注入提示词的站内作品候选（json tag 参与提示词内容，属于外部契约）.
type PromptVideoCandidate struct {
	VideoID       uint              `json:"video_id"`
	Title         string            `json:"title"`
	Description   string            `json:"description,omitempty"`
	Partition     string            `json:"partition,omitempty"`
	Cover         string            `json:"cover,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Route         string            `json:"route"`
	WorkType      string            `json:"work_type"`
	PartSummaries []PromptVideoPart `json:"parts,omitempty"`
}

// PromptVideoCandidateSet 一轮召回的候选集合与来源。
type PromptVideoCandidateSet struct {
	RetrievalMode string
	QueryKeywords []string
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
// 规则与原 buildPromptVideoCandidate 一致：描述截断 240 字，
// 分 P 多于一个时标记为 multi_part 并逐个生成路由。
func NewPromptVideoCandidate(input VideoCandidateInput) PromptVideoCandidate {
	item := PromptVideoCandidate{
		VideoID:     input.VideoID,
		Title:       strings.TrimSpace(input.Title),
		Description: TrimPromptText(input.Description, maxDescriptionRunes),
		Partition:   strings.TrimSpace(input.Partition),
		Cover:       strings.TrimSpace(input.Cover),
		Route:       BuildVideoRoute(input.VideoID, 0),
		WorkType:    "single",
		Tags:        normalizeTagNames(input.Tags),
	}

	if len(input.Parts) > 1 {
		item.WorkType = "multi_part"

		item.PartSummaries = make([]PromptVideoPart, 0, len(input.Parts))

		for idx, part := range input.Parts {
			p := idx + 1
			item.PartSummaries = append(item.PartSummaries, PromptVideoPart{
				P:     p,
				Title: strings.TrimSpace(part.Title),
				Route: BuildVideoRoute(input.VideoID, p),
			})
		}
	}

	return item
}

// BuildPromptVideoContext 拼装「站内作品候选上下文」提示词段。
func BuildPromptVideoContext(maxWorks int, candidateSet PromptVideoCandidateSet) (string, error) {
	var builder strings.Builder

	builder.WriteString("【站内作品候选上下文】\n")

	if len(candidateSet.QueryKeywords) > 0 {
		builder.WriteString("- 第一轮 AI 提炼出的搜索关键词：")
		builder.WriteString(strings.Join(candidateSet.QueryKeywords, "、"))
		builder.WriteString("\n")
	}

	builder.WriteString("- 这是已经判定为“找视频/找作品”后的第二轮正式回答。\n")

	switch candidateSet.RetrievalMode {
	case RetrievalModeVector:
		builder.WriteString("- 当前候选来源：语义向量召回结果，请优先从这些作品中选择最相关的内容。\n")
	case RetrievalModeKeyword:
		builder.WriteString("- 当前候选来源：关键词命中结果，请优先从这些作品中选择最相关的内容。\n")
	case RetrievalModeLatest:
		builder.WriteString("- 当前候选来源：关键词未命中任何已发布作品，后端已按最新发布时间回退提供候选作品。你需要自行判断是否值得推荐；如果都不合适，就明确说暂时没有特别匹配的站内结果。\n")
	default:
		builder.WriteString("- 当前候选来源：没有候选作品。\n")
	}

	builder.WriteString("- 本轮作品候选上限：")
	builder.WriteString(strconv.Itoa(maxWorks))
	builder.WriteString("（按作品计数，不按分P计数）\n")
	builder.WriteString("- 找视频时最多返回 ")
	builder.WriteString(strconv.Itoa(MaxVideoResultLinkCount))
	builder.WriteString(" 个链接结果。\n")

	if len(candidateSet.Candidates) == 0 {
		builder.WriteString("- 当前没有候选作品。如果用户是在找站内视频，请直接说明暂时没找到，不要编造标题、ID、分P序号或路由。")

		return builder.String(), nil
	}

	raw, err := json.MarshalIndent(candidateSet.Candidates, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化站内作品候选失败: %w", err)
	}

	builder.WriteString("- 以下 JSON 是本轮唯一允许引用的站内作品、tag 和路由；如果要返回链接，必须原样使用其中的 route：\n")
	builder.Write(raw)

	return builder.String(), nil
}

// BuildVideoRoute 生成前端作品路由；p > 0 时附带分 P 序号。
func BuildVideoRoute(videoID uint, p int) string {
	if videoID == 0 {
		return ""
	}

	route := "/video/" + strconv.FormatUint(uint64(videoID), 10)
	if p > 0 {
		route += "/" + strconv.Itoa(p)
	}

	return route
}

// NormalizeAnalysisKeywords 规范化意图分析关键词：去空、去超长、忽略大小写去重、截断到上限。
func NormalizeAnalysisKeywords(keywords []string) []string {
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
func EnsurePositiveLimit(limit int) int {
	if limit > 0 {
		return limit
	}

	return DefaultCandidateLimit
}

// TrimPromptText 按字符数截断文本，超长时追加省略号。
func TrimPromptText(value string, maxRunes int) string {
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

// normalizeTagNames 过滤空白 tag 名；无有效值时返回 nil（保持 omitempty 语义）。
func normalizeTagNames(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	names := make([]string, 0, len(tags))

	for _, tag := range tags {
		name := strings.TrimSpace(tag)
		if name != "" {
			names = append(names, name)
		}
	}

	return names
}
