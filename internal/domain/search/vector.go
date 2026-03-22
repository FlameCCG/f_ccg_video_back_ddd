package search

import "strings"

// 封面图参与向量化的模式。取值与 AI 配置 embeddingImageMode 逐字一致。
const (
	// EmbeddingImageModeAuto 有可用封面时写入图像向量，否则只使用文本。
	EmbeddingImageModeAuto = "auto"
	// EmbeddingImageModeOn 封面不可用时索引失败。
	EmbeddingImageModeOn = "on"
	// EmbeddingImageModeOff 不读取封面。
	EmbeddingImageModeOff = "off"
)

// VectorPolicy 是视频向量文本与封面模式的业务规则。
//
// 零值即可使用。文档 _id 和索引名由基础设施负责；多来源排名融合由 RankFusion 处理。
type VectorPolicy struct{}

func (VectorPolicy) RecallCap(configured int) int {
	if configured > 0 {
		return configured
	}
	return 8
}

// RecallLimit 沿用站点召回上限：未指定时使用配置，超过配置时截断。
func (VectorPolicy) RecallLimit(requested, configured int) int {
	if requested <= 0 || configured > 0 && requested > configured {
		return configured
	}
	return requested
}

// TrimNonEmpty 去掉首尾空白并丢弃空串；全空时返回 nil.
func (VectorPolicy) TrimNonEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))

	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

// NormalizeImageMode 把配置里的封面模式收成 auto / on / off；无法识别时回落 auto。
func (VectorPolicy) NormalizeImageMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case EmbeddingImageModeOn:
		return EmbeddingImageModeOn
	case EmbeddingImageModeOff:
		return EmbeddingImageModeOff
	default:
		return EmbeddingImageModeAuto
	}
}

// VideoVectorInput 是构造视频向量文本内容所需的业务字段.
type VideoVectorInput struct {
	Title       string
	Description string
	Partition   string
	Tags        []string
	PartTitles  []string
}

// BuildContent 拼装送入 embedding 的文本内容。
//
// 字段顺序与分隔符属于向量库内容契约：重建索引时必须与检索侧一致，
// 否则同一视频在新旧向量空间的表示会漂移。
func (in VideoVectorInput) BuildContent() string {
	policy := VectorPolicy{}
	contentParts := []string{
		"标题: " + strings.TrimSpace(in.Title),
		"描述: " + strings.TrimSpace(in.Description),
		"分区: " + strings.TrimSpace(in.Partition),
	}

	if tags := policy.TrimNonEmpty(in.Tags); len(tags) > 0 {
		contentParts = append(contentParts, "标签: "+strings.Join(tags, "、"))
	}

	if parts := policy.TrimNonEmpty(in.PartTitles); len(parts) > 0 {
		contentParts = append(contentParts, "分P: "+strings.Join(parts, "、"))
	}

	return strings.Join(contentParts, "\n")
}
