package ai

import (
	"context"
	"strings"

	aidomain "f_ccg_video_back_ddd/internal/domain/ai"

	"go.uber.org/zap"
)

const vectorQueryMaxChars = 1000

// loadPromptVideoCandidates 为搜索工具组织混合召回、数据库回退及候选详情回查。
func (s *Service) loadPromptVideoCandidates(
	ctx context.Context,
	conv conversation,
	keywords []string,
	limit int,
) (aidomain.PromptVideoCandidateSet, error) {
	if s.prompts == nil || !s.prompts.Available() {
		return aidomain.PromptVideoCandidateSet{}, nil
	}

	keywords = (aidomain.VideoQueryAnalysis{}).NormalizeAnalysisKeywords(keywords)
	images := conv.queryImages()
	if len(keywords) == 0 && len(images) == 0 && strings.TrimSpace(conv.LatestUserText) == "" {
		return aidomain.PromptVideoCandidateSet{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ids, retrievalMode := s.searchPromptVideoIDs(ctx, conv, keywords, limit)
	if len(ids) == 0 {
		latestIDs, err := s.prompts.LatestPublishedIDs(ctx, (aidomain.PromptVideoCandidateSet{}).EnsurePositiveLimit(limit))
		if err != nil {
			return aidomain.PromptVideoCandidateSet{}, err
		}
		if len(latestIDs) == 0 {
			return aidomain.PromptVideoCandidateSet{
				RetrievalMode: aidomain.RetrievalModeLatest,
				QueryKeywords: keywords,
				UsedImage:     len(images) > 0,
			}, nil
		}

		return s.hydratePromptVideoCandidates(
			ctx,
			latestIDs, aidomain.RetrievalModeLatest, keywords,
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
) (aidomain.PromptVideoCandidateSet, error) {
	inputs, err := s.prompts.Load(ctx, ids)
	if err != nil {
		return aidomain.PromptVideoCandidateSet{}, err
	}

	items := make([]aidomain.PromptVideoCandidate, 0, len(inputs))
	for _, input := range inputs {
		items = append(items, aidomain.NewPromptVideoCandidate(input))
	}

	return aidomain.PromptVideoCandidateSet{
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
	limit = (aidomain.PromptVideoCandidateSet{}).EnsurePositiveLimit(limit)
	images := conv.queryImages()
	input := VideoVectorQuery{
		Text: (aidomain.PromptVideoCandidate{}).TrimPromptText(strings.TrimSpace(conv.LatestUserText), vectorQueryMaxChars),
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
			s.logger.Warn("search video vector index failed, fallback to keyword match", zap.Error(err))
		case len(ids) > 0 && input.ImageURL != "":
			return ids, aidomain.RetrievalModeImageVector
		case len(ids) > 0:
			return ids, aidomain.RetrievalModeVector
		}
	}

	// query 的长度预算与模型提取的短关键词不同，缺少关键词时保留完整查询用于 SQL 匹配。
	if len(keywords) == 0 && input.Text != "" {
		keywords = []string{input.Text}
	}
	ids, err := s.prompts.KeywordSearchIDs(ctx, keywords, limit)
	if err != nil {
		s.logger.Warn("keyword match for AI video search failed", zap.Error(err))

		return nil, aidomain.RetrievalModeKeyword
	}

	return ids, aidomain.RetrievalModeKeyword
}
