package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/ai"
	"f_ccg_video_back_ddd/internal/infrastructure/config"

	"go.uber.org/zap"
)

// buildResponsesSystemPrompt 组装本轮系统提示词。
//
// 两轮流程与原实现一致：先用小 token 预算做「是否在找站内视频」的意图分析，
// 命中则召回候选作品并把候选上下文追加到后台系统提示词之后。
// 任一环节失败都返回 basePrompt + 错误，由调用方仅告警、不中断正式回答。
func (s *Service) buildResponsesSystemPrompt(
	ctx context.Context,
	cfg *config.AI,
	input json.RawMessage,
	model string,
) (string, error) {
	basePrompt := strings.TrimSpace(cfg.SystemPrompt)
	if !domain.HasRawJSON(input) {
		return basePrompt, nil
	}

	analysis, err := s.analyzeVideoQuery(ctx, cfg, domain.FirstNonEmpty(model, cfg.ChatModel), input)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			zap.L().Debug("skip AI video query analysis after context deadline", zap.Error(err))

			return basePrompt, nil
		}

		return basePrompt, err
	}

	if len(analysis.Keywords) == 0 {
		return basePrompt, nil
	}

	limit := resolvePromptCandidateLimit(cfg)

	candidateSet, err := s.loadPromptVideoCandidates(ctx, analysis.Keywords, limit)
	if err != nil {
		return basePrompt, err
	}

	contextPrompt, err := domain.BuildPromptVideoContext(limit, candidateSet)
	if err != nil {
		return basePrompt, err
	}

	if contextPrompt == "" {
		return basePrompt, nil
	}

	if basePrompt == "" {
		return contextPrompt, nil
	}

	return basePrompt + "\n\n" + contextPrompt, nil
}

// analyzeVideoQuery 第一轮意图分析：让聊天模型提炼站内检索关键词。
func (s *Service) analyzeVideoQuery(
	ctx context.Context,
	cfg *config.AI,
	model string,
	input json.RawMessage,
) (*domain.VideoQueryAnalysis, error) {
	analysisInput, err := domain.InjectSystemPrompt(input, domain.VideoQueryAnalysisPrompt)
	if err != nil {
		return nil, err
	}

	analysisCtx, cancel := context.WithTimeout(ctx, resolveAnalysisTimeout(cfg))
	defer cancel()

	text, _, err := s.text.InvokeText(analysisCtx, TextInvocation{
		Model:     model,
		Input:     analysisInput,
		MaxTokens: domain.AnalysisMaxOutputTokens,
	})
	if err != nil {
		return nil, err
	}

	var analysis domain.VideoQueryAnalysis
	if err = json.Unmarshal([]byte(text), &analysis); err != nil {
		return nil, fmt.Errorf("解析视频查询分析结果失败: %w", err)
	}

	zap.L().Info("解析视频查询分析结果", zap.Any("analysis", analysis))
	analysis.Keywords = domain.NormalizeAnalysisKeywords(analysis.Keywords)

	return &analysis, nil
}

// loadPromptVideoCandidates 按关键词召回站内候选作品；未命中时回退最新发布。
func (s *Service) loadPromptVideoCandidates(
	ctx context.Context,
	keywords []string,
	limit int,
) (domain.PromptVideoCandidateSet, error) {
	if s.prompts == nil || !s.prompts.Available() {
		return domain.PromptVideoCandidateSet{}, nil
	}

	keywords = domain.NormalizeAnalysisKeywords(keywords)
	if len(keywords) == 0 {
		return domain.PromptVideoCandidateSet{}, nil
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	ids, retrievalMode, err := s.searchPromptVideoIDs(ctx, keywords, limit)
	if err != nil {
		return domain.PromptVideoCandidateSet{}, err
	}

	if len(ids) == 0 {
		ids, err = s.prompts.LatestPublishedIDs(ctx, domain.EnsurePositiveLimit(limit))
		if err != nil {
			return domain.PromptVideoCandidateSet{}, err
		}

		if len(ids) == 0 {
			return domain.PromptVideoCandidateSet{
				RetrievalMode: domain.RetrievalModeLatest,
				QueryKeywords: keywords,
			}, nil
		}

		return s.hydratePromptVideoCandidates(ctx, ids, domain.RetrievalModeLatest, keywords)
	}

	return s.hydratePromptVideoCandidates(ctx, ids, retrievalMode, keywords)
}

// hydratePromptVideoCandidates 按 ID 顺序加载并构造候选作品。
func (s *Service) hydratePromptVideoCandidates(
	ctx context.Context,
	ids []uint,
	retrievalMode string,
	keywords []string,
) (domain.PromptVideoCandidateSet, error) {
	inputs, err := s.prompts.Load(ctx, ids)
	if err != nil {
		return domain.PromptVideoCandidateSet{}, err
	}

	items := make([]domain.PromptVideoCandidate, 0, len(inputs))
	for _, input := range inputs {
		items = append(items, domain.NewPromptVideoCandidate(input))
	}

	return domain.PromptVideoCandidateSet{
		RetrievalMode: retrievalMode,
		QueryKeywords: keywords,
		Candidates:    items,
	}, nil
}

// searchPromptVideoIDs 先走语义向量召回，失败或空结果时回退关键词命中。
func (s *Service) searchPromptVideoIDs(
	ctx context.Context,
	keywords []string,
	limit int,
) ([]uint, string, error) {
	limit = domain.EnsurePositiveLimit(limit)

	if s.vectors != nil {
		ids, err := s.vectors.SearchVideoVectorIDs(ctx, strings.Join(keywords, " "), limit)
		if err == nil && len(ids) > 0 {
			return ids, domain.RetrievalModeVector, nil
		}

		if err != nil {
			zap.L().Warn("search video vector index failed, fallback to keyword match", zap.Error(err))
		}
	}

	ids, err := s.prompts.KeywordSearchIDs(ctx, keywords, limit)
	if err != nil {
		return nil, domain.RetrievalModeKeyword, err
	}

	return ids, domain.RetrievalModeKeyword, nil
}

// resolvePromptCandidateLimit 使用 vectorTopK 作为 AI 找视频候选上限（向量/关键词/最新回退共用）。
func resolvePromptCandidateLimit(cfg *config.AI) int {
	if cfg != nil && cfg.VectorTopK > 0 {
		return cfg.VectorTopK
	}

	return domain.DefaultCandidateLimit
}

// resolveAnalysisTimeout 意图分析超时：默认 20s，配置更小时取配置值。
func resolveAnalysisTimeout(cfg *config.AI) time.Duration {
	timeout := domain.DefaultAnalysisTimeoutSec * time.Second
	if cfg == nil || cfg.TimeoutSec <= 0 {
		return timeout
	}

	cfgTimeout := time.Duration(cfg.TimeoutSec) * time.Second
	if cfgTimeout < timeout {
		return cfgTimeout
	}

	return timeout
}
