package ai

import (
	"context"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/pkg/logctx"
	"fmt"
	"strings"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/ai"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

const (
	promptBuildBudget              = 30 * time.Second
	analysisHistoryMessages        = 6
	analysisHistoryCharsPerMessage = 400
	vectorQueryMaxChars            = 1000
)

// buildResponsesSystemPrompt 在正式回答前判断找视频意图并注入站内候选。
// 整条提示词构建链受统一预算约束，超时降级为普通聊天，避免无限延迟首 token。
func (s *Service) buildResponsesSystemPrompt(
	ctx context.Context,
	cfg *config.AI,
	runtime RuntimeConfig,
	conv conversation,
	model string,
) (string, error) {
	basePrompt := strings.TrimSpace(cfg.SystemPrompt)
	if !conv.HasMessages() {
		return basePrompt, nil
	}
	// 无法加载站内候选时，意图分析也无法影响回答；省掉一次额外模型调用。
	if s.prompts == nil || !s.prompts.Available() {
		return basePrompt, nil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	promptCtx, cancel := context.WithTimeout(ctx, promptBuildBudget)
	defer cancel()

	analysis, err := s.analyzeVideoQuery(
		promptCtx,
		cfg,
		runtime,
		domain.FirstNonEmpty(model, cfg.ChatModel),
		conv,
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			logctx.With(ctx, s.logger).Debug("skip AI video query analysis after context deadline", zap.String("event", "ai.prompt_service.build_responses_system_prompt.diagnostic"), zap.String("operation", "ai.prompt_service.build_responses_system_prompt"), zap.Error(err))

			return basePrompt, nil
		}

		return basePrompt, err
	}

	if !analysis.WantsVideoSearch() {
		return basePrompt, nil
	}

	limit := resolvePromptCandidateLimit(cfg)

	candidateSet, err := s.loadPromptVideoCandidates(promptCtx, conv, analysis.Keywords, limit)
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

func (s *Service) analyzeVideoQuery(
	ctx context.Context,
	cfg *config.AI,
	runtime RuntimeConfig,
	model string,
	conv conversation,
) (domain.VideoQueryAnalysis, error) {
	var analysis domain.VideoQueryAnalysis

	transcript := conv.transcript(analysisHistoryMessages, analysisHistoryCharsPerMessage)
	if strings.TrimSpace(transcript) == "" {
		return analysis, nil
	}

	if len(conv.queryImages()) > 0 {
		transcript += "\n（用户本轮还上传了图片）"
	}

	analysisCtx, cancel := context.WithTimeout(ctx, resolveAnalysisTimeout(cfg))
	defer cancel()

	text, err := s.text.InvokeText(analysisCtx, TextInvocation{
		Runtime: runtime,
		Model:   model,
		Messages: []*schema.Message{
			schema.SystemMessage(domain.VideoQueryAnalysisPrompt),
			schema.UserMessage(transcript),
		},
		MaxTokens: domain.AnalysisMaxOutputTokens,
	})
	if err != nil {
		return analysis, err
	}

	if err = json.Unmarshal([]byte(extractJSONObject(text)), &analysis); err != nil {
		return analysis, fmt.Errorf("解析视频查询分析结果失败: %w", err)
	}

	analysis.Keywords = domain.NormalizeAnalysisKeywords(analysis.Keywords)
	logctx.With(ctx, s.logger).Debug(
		"video query analysis", zap.String("event", "ai.prompt_service.analyze_video_query.diagnostic"), zap.String("operation", "ai.prompt_service.analyze_video_query"),
		zap.String("intent", analysis.Intent),
		zap.Strings("keywords", analysis.Keywords),
	)

	return analysis, nil
}

func extractJSONObject(text string) string {
	text = strings.TrimSpace(text)
	start := strings.Index(text, "{")

	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return text
	}

	return text[start : end+1]
}

func (s *Service) loadPromptVideoCandidates(
	ctx context.Context,
	conv conversation,
	keywords []string,
	limit int,
) (domain.PromptVideoCandidateSet, error) {
	if s.prompts == nil || !s.prompts.Available() {
		return domain.PromptVideoCandidateSet{}, nil
	}

	keywords = domain.NormalizeAnalysisKeywords(keywords)

	images := conv.queryImages()
	if len(keywords) == 0 && len(images) == 0 {
		return domain.PromptVideoCandidateSet{}, nil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	ids, retrievalMode := s.searchPromptVideoIDs(ctx, conv, keywords, limit)
	if len(ids) == 0 {
		latestIDs, err := s.prompts.LatestPublishedIDs(ctx, domain.EnsurePositiveLimit(limit))
		if err != nil {
			return domain.PromptVideoCandidateSet{}, err
		}

		if len(latestIDs) == 0 {
			return domain.PromptVideoCandidateSet{
				RetrievalMode: domain.RetrievalModeLatest,
				QueryKeywords: keywords,
				UsedImage:     len(images) > 0,
			}, nil
		}

		return s.hydratePromptVideoCandidates(
			ctx,
			latestIDs,
			domain.RetrievalModeLatest,
			keywords,
			len(images) > 0,
		)
	}

	return s.hydratePromptVideoCandidates(
		ctx,
		ids,
		retrievalMode,
		keywords,
		len(images) > 0,
	)
}

func (s *Service) hydratePromptVideoCandidates(
	ctx context.Context,
	ids []uint,
	retrievalMode string,
	keywords []string,
	usedImage bool,
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
		UsedImage:     usedImage,
		Candidates:    items,
	}, nil
}

// searchPromptVideoIDs 先使用用户原句和首张图片做多模态召回，再退到 SQL 关键词匹配。
func (s *Service) searchPromptVideoIDs(
	ctx context.Context,
	conv conversation,
	keywords []string,
	limit int,
) ([]uint, string) {
	limit = domain.EnsurePositiveLimit(limit)
	images := conv.queryImages()

	input := VideoVectorQuery{
		Text: domain.TrimPromptText(strings.TrimSpace(conv.LatestUserText), vectorQueryMaxChars),
	}
	if len(images) > 0 {
		input.ImageURL = images[0]
	}

	if input.Text == "" && len(keywords) > 0 {
		input.Text = strings.Join(keywords, " ")
	}

	if s.vectors != nil && !input.IsZero() {
		ids, err := s.vectors.SearchVideoVectorIDsMultiModal(ctx, input, limit)
		switch {
		case err != nil:
			logctx.With(ctx, s.logger).Warn("search video vector index failed, fallback to keyword match", zap.String("event", "ai.prompt_service.search_prompt_video_i_ds.degraded"), zap.String("operation", "ai.prompt_service.search_prompt_video_i_ds"), zap.Error(err))
		case len(ids) > 0 && input.ImageURL != "":
			return ids, domain.RetrievalModeImageVector
		case len(ids) > 0:
			return ids, domain.RetrievalModeVector
		}
	}

	ids, err := s.prompts.KeywordSearchIDs(ctx, keywords, limit)
	if err != nil {
		logctx.With(ctx, s.logger).Warn("keyword match for AI video search failed", zap.String("event", "ai.prompt_service.search_prompt_video_i_ds.degraded"), zap.String("operation", "ai.prompt_service.search_prompt_video_i_ds"), zap.Error(err))

		return nil, domain.RetrievalModeKeyword
	}

	return ids, domain.RetrievalModeKeyword
}

func resolvePromptCandidateLimit(cfg *config.AI) int {
	if cfg != nil && cfg.VectorTopK > 0 {
		return cfg.VectorTopK
	}

	return domain.DefaultCandidateLimit
}

func resolveAnalysisTimeout(cfg *config.AI) time.Duration {
	timeout := domain.DefaultAnalysisTimeoutSec * time.Second
	if cfg == nil || cfg.TimeoutSec <= 0 {
		return timeout
	}

	configured := time.Duration(cfg.TimeoutSec) * time.Second
	if configured < timeout {
		return configured
	}

	return timeout
}
