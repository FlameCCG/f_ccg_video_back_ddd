package ai

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	aidomain "f_ccg_video_back_ddd/internal/domain/ai"
)

// promptVideoContext 维护提示词的 JSON 和前端路由契约，领域只提供候选事实与引用口径。
type promptVideoContext struct{}

// VideoQueryAnalysis 是上游意图响应 DTO；名称保留既有 JSON 类型错误文案。
type VideoQueryAnalysis struct {
	Intent   string   `json:"intent"`
	Keywords []string `json:"keywords"`
}

type promptVideoPartPayload struct {
	P     int    `json:"p"`
	Title string `json:"title"`
	Route string `json:"route"`
}

type promptVideoCandidatePayload struct {
	VideoID       uint                     `json:"video_id"`
	Title         string                   `json:"title"`
	Description   string                   `json:"description,omitempty"`
	Partition     string                   `json:"partition,omitempty"`
	Cover         string                   `json:"cover,omitempty"`
	Tags          []string                 `json:"tags,omitempty"`
	Route         string                   `json:"route"`
	WorkType      string                   `json:"work_type"`
	PartSummaries []promptVideoPartPayload `json:"parts,omitempty"`
}

func (f promptVideoContext) candidate(value aidomain.PromptVideoCandidate) promptVideoCandidatePayload {
	out := promptVideoCandidatePayload{
		VideoID: value.VideoID, Title: value.Title, Description: value.Description,
		Partition: value.Partition, Cover: value.Cover, Tags: value.Tags,
		Route: f.videoRoute(value.VideoID, 0), WorkType: value.WorkType,
	}
	if value.PartSummaries != nil {
		out.PartSummaries = make([]promptVideoPartPayload, len(value.PartSummaries))
		for i, part := range value.PartSummaries {
			out.PartSummaries[i] = promptVideoPartPayload{
				P: part.P, Title: part.Title, Route: f.videoRoute(value.VideoID, part.P),
			}
		}
	}
	return out
}

// Build 拼装「站内作品候选上下文」提示词段，保持既有文案及 JSON 格式。
func (f promptVideoContext) Build(set aidomain.PromptVideoCandidateSet, maxWorks int) (string, error) {
	var builder strings.Builder

	builder.WriteString("【站内作品候选上下文】\n")

	if len(set.QueryKeywords) > 0 {
		builder.WriteString("- 第一轮 AI 提炼出的搜索关键词：")
		builder.WriteString(strings.Join(set.QueryKeywords, "、"))
		builder.WriteString("\n")
	}

	builder.WriteString("- 这是已经判定为“找视频/找作品”后的第二轮正式回答。\n")

	builder.WriteString(set.RetrievalGuidance())

	builder.WriteString("- 本轮作品候选上限：")
	builder.WriteString(strconv.Itoa(maxWorks))
	builder.WriteString("（按作品计数，不按分P计数）\n")
	builder.WriteString("- 找视频时最多返回 ")
	builder.WriteString(strconv.Itoa(aidomain.MaxVideoResultLinkCount))
	builder.WriteString(" 个链接结果。\n")

	if len(set.Candidates) == 0 {
		builder.WriteString("- 当前没有候选作品。如果用户是在找站内视频，请直接说明暂时没找到，不要编造标题、ID、分P序号或路由。")

		return builder.String(), nil
	}

	candidates := make([]promptVideoCandidatePayload, len(set.Candidates))
	for i, candidate := range set.Candidates {
		candidates[i] = f.candidate(candidate)
	}
	raw, err := json.MarshalIndent(candidates, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化站内作品候选失败: %w", err)
	}

	builder.WriteString("- 以下 JSON 是本轮唯一允许引用的站内作品、tag 和路由；如果要返回链接，必须原样使用其中的 route：\n")
	builder.Write(raw)

	return builder.String(), nil
}

// videoRoute 生成前端作品路由；p > 0 时附带分 P 序号。
func (promptVideoContext) videoRoute(videoID uint, p int) string {
	if videoID == 0 {
		return ""
	}

	route := "/video/" + strconv.FormatUint(uint64(videoID), 10)
	if p > 0 {
		route += "/" + strconv.Itoa(p)
	}

	return route
}
