package search

import (
	"encoding/json"
	"strings"

	app "f_ccg_video_back_ddd/internal/application/search"
	searchdomain "f_ccg_video_back_ddd/internal/domain/search"

	"github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

// parseVideoHits 解析视频索引的一页响应；分片级错误只告警并返回零值。
func (a Adapter) parseVideoHits(res *elastic.SearchResult, logger *zap.Logger) app.VideoHits {
	if logger == nil {
		logger = zap.NewNop()
	}

	if res == nil || res.Error != nil {
		if res != nil && res.Error != nil {
			logger.Warn("video search error", zap.Any("error", res.Error))
		}

		return app.VideoHits{}
	}

	return app.VideoHits{Total: res.TotalHits(), Items: a.parseVideoList(res)}
}

// parseUserHits 解析用户索引的一页响应；分片级错误只告警并返回零值。
func (a Adapter) parseUserHits(res *elastic.SearchResult, logger *zap.Logger) app.UserHits {
	if logger == nil {
		logger = zap.NewNop()
	}

	if res == nil || res.Error != nil {
		if res != nil && res.Error != nil {
			logger.Warn("user search error", zap.Any("error", res.Error))
		}

		return app.UserHits{}
	}

	return app.UserHits{Total: res.TotalHits(), Items: a.parseUserList(res)}
}

// parseVideoList 把 ES 命中解码为视频结果项，兼容 snake_case 与 camelCase 字段名。
func (a Adapter) parseVideoList(res *elastic.SearchResult) []app.SearchVideoHit {
	if res == nil || res.Error != nil || res.Hits == nil {
		return nil
	}

	list := make([]app.SearchVideoHit, 0, len(res.Hits.Hits))

	for _, hit := range res.Hits.Hits {
		var data map[string]any
		if err := json.Unmarshal(hit.Source, &data); err != nil {
			continue
		}

		authorUsername := a.toString(data["author_username"])
		if authorUsername == "" {
			authorUsername = a.toString(data["authorUsername"])
		}

		createdAt := a.toTime(data["created_at"])
		if createdAt.IsZero() {
			createdAt = a.toTime(data["createdAt"])
		}

		list = append(list, app.SearchVideoHit{
			ID:             a.toUint(data["id"]),
			Title:          a.toString(data["title"]),
			Cover:          a.toString(data["cover"]),
			AuthorUsername: authorUsername,
			CreatedAt:      createdAt,
			Highlight:      a.toHighlightMap(hit.Highlight),
			Views:          a.toInt64(data["views"]),
			DanmuCount:     a.toInt64(data["danmu_count"]),
			Duration:       a.toInt(data["duration"]),
			Tags:           a.toStringSlice(data["tags"]),
		})
	}

	return list
}

// parseUserList 把 ES 命中解码为用户结果项。
func (a Adapter) parseUserList(res *elastic.SearchResult) []app.SearchUserHit {
	if res == nil || res.Error != nil || res.Hits == nil {
		return nil
	}

	list := make([]app.SearchUserHit, 0, len(res.Hits.Hits))

	for _, hit := range res.Hits.Hits {
		var data map[string]any
		if err := json.Unmarshal(hit.Source, &data); err != nil {
			continue
		}

		list = append(list, app.SearchUserHit{
			ID:            a.toUint(data["id"]),
			Username:      a.toString(data["username"]),
			Avatar:        a.toString(data["avatar"]),
			Highlight:     a.toHighlightMap(hit.Highlight),
			Level:         a.toInt(data["level"]),
			FollowerCount: a.toInt64(data["follower_count"]),
		})
	}

	return list
}

// parseSuggestHits 从命中文档里抽取联想候选值并去重（大小写不敏感）。
//
// ES 拼音命中时原文可能不含查询词，此处仍然保留，由 _score 保证相关性。
func (a Adapter) parseSuggestHits(resp *elastic.SearchResult, docType, prefix string, size int) []app.SuggestionItem {
	if resp == nil || resp.Hits == nil || len(resp.Hits.Hits) == 0 {
		return []app.SuggestionItem{}
	}

	items := make([]app.SuggestionItem, 0, size)
	seen := make(map[string]struct{}, size)
	keywords := searchdomain.KeywordPolicy{}

	appendValue := func(value string) bool {
		value = strings.TrimSpace(value)
		if value == "" {
			return false
		}

		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			return false
		}

		seen[key] = struct{}{}
		items = append(items, app.SuggestionItem{
			Value:     value,
			Highlight: (app.SuggestionHighlighter{}).HighlightSuggestion(value, prefix),
			DocType:   docType,
		})

		return len(items) >= size
	}

	for _, hit := range resp.Hits.Hits {
		if len(items) >= size {
			break
		}

		var data map[string]any
		if err := json.Unmarshal(hit.Source, &data); err != nil {
			continue
		}

		switch docType {
		case searchdomain.DocTypeUser:
			if appendValue(a.toString(data["username"])) {
				return items
			}
		case searchdomain.DocTypeTag:
			for _, tag := range keywords.SelectMatchingTags(a.toStringSlice(data["tags"]), prefix) {
				if appendValue(tag) {
					return items
				}
			}
		default:
			if appendValue(a.toString(data["title"])) {
				return items
			}
		}
	}

	return items
}

// parseMentionSearchHitUserID 从命中项取用户 ID：优先解析 _id（users:12），其次取 fields.id。
func (a Adapter) parseMentionSearchHitUserID(hit *elastic.SearchHit) uint {
	if hit == nil {
		return 0
	}

	if id, ok := (esContract{}).ParseDocumentID(hit.Id); ok {
		return id
	}

	return a.toUint(hit.Fields["id"])
}
