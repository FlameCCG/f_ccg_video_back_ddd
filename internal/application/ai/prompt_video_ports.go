package ai

import (
	"context"
	"strings"

	aivo "f_ccg_video_back_ddd/internal/domain/ai/valueobject"
)

// PromptVideoQuery 站内作品候选读侧投影端口。
//
// 读侧投影允许查询 videos / video_parts / tags 表，实现在本上下文的 infrastructure/persistence/ai；视频写侧归 video 上下文。
type PromptVideoQuery interface {
	// Available 报告底层数据库是否可用；不可用时应用服务跳过候选注入。
	Available() bool
	// KeywordSearchIDs 关键词命中已发布作品 ID（按匹配优先级 + 发布时间排序）。
	KeywordSearchIDs(ctx context.Context, keywords []string, limit int) ([]uint, error)
	// LatestPublishedIDs 最新发布作品 ID（关键词未命中时的回退）。
	LatestPublishedIDs(ctx context.Context, limit int) ([]uint, error)
	// Load 按给定 ID 顺序加载候选作品原始数据（缺失的 ID 直接跳过）。
	Load(ctx context.Context, ids []uint) ([]aivo.VideoCandidateInput, error)
}

// VideoVectorSearch 站内视频向量召回端口（跨上下文）。
//
// 由 search 上下文的应用服务在 bootstrap 中绑定实现。
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
