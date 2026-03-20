package search

import (
	"context"
	"strings"

	"go.uber.org/zap"
)

// hydrateVideoList 用库内数据回填 ES 文档中缺失的作者名与创建时间。
func (s *SearchService) hydrateVideoList(ctx context.Context, hits []SearchVideoHit) {
	if len(hits) == 0 || !s.fallback.Available() {
		return
	}

	idSet := make(map[uint]struct{}, len(hits))

	ids := make([]uint, 0, len(hits))

	for _, item := range hits {
		if item.ID == 0 || (!item.CreatedAt.IsZero() && item.AuthorUsername != "") {
			continue
		}

		if _, ok := idSet[item.ID]; ok {
			continue
		}

		idSet[item.ID] = struct{}{}
		ids = append(ids, item.ID)
	}

	if len(ids) == 0 {
		return
	}

	rows, err := s.fallback.VideoMetaByIDs(ctx, ids)
	if err != nil {
		s.logger.Warn("hydrate search videos failed", zap.Error(err))

		return
	}

	metaByID := make(map[uint]VideoSearchMeta, len(rows))
	for _, row := range rows {
		metaByID[row.ID] = row
	}

	applyVideoMeta(hits, metaByID)
}

// applyVideoMeta 就地回填非空的作者名与创建时间。
func applyVideoMeta(hits []SearchVideoHit, metaByID map[uint]VideoSearchMeta) {
	for i := range hits {
		meta, ok := metaByID[hits[i].ID]
		if !ok {
			continue
		}

		if meta.AuthorUsername != "" {
			hits[i].AuthorUsername = meta.AuthorUsername
		}

		if !meta.CreatedAt.IsZero() {
			hits[i].CreatedAt = meta.CreatedAt
		}
	}
}

// buildRelatedTags 从视频命中的标签中去重派生相关标签（最多 10 个）。
func buildRelatedTags(videos []SearchVideoHit) []SearchTagHit {
	seen := make(map[string]struct{})
	tags := make([]SearchTagHit, 0)

	for _, video := range videos {
		for _, tag := range video.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}

			if _, ok := seen[tag]; ok {
				continue
			}

			seen[tag] = struct{}{}

			tags = append(tags, SearchTagHit{Name: tag})

			if len(tags) >= relatedTagLimit {
				return tags
			}
		}
	}

	return tags
}
