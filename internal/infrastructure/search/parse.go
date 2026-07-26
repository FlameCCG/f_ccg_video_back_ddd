package search

import (
	"encoding/json"
	"strconv"
	"strings"

	app "f_ccg_video_back_ddd/internal/application/search"
	domain "f_ccg_video_back_ddd/internal/domain/search"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"

	"github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

// parseVideoHits 解析视频索引的一页响应；分片级错误只告警并返回零值（与原实现一致）。
func parseVideoHits(res *elastic.SearchResult) app.VideoHits {
	if res == nil || res.Error != nil {
		if res != nil && res.Error != nil {
			zap.L().Warn("video search error", zap.Any("error", res.Error))
		}

		return app.VideoHits{}
	}

	return app.VideoHits{Total: res.TotalHits(), Items: parseVideoList(res)}
}

// parseUserHits 解析用户索引的一页响应；分片级错误只告警并返回零值（与原实现一致）。
func parseUserHits(res *elastic.SearchResult) app.UserHits {
	if res == nil || res.Error != nil {
		if res != nil && res.Error != nil {
			zap.L().Warn("user search error", zap.Any("error", res.Error))
		}

		return app.UserHits{}
	}

	return app.UserHits{Total: res.TotalHits(), Items: parseUserList(res)}
}

// parseVideoList 把 ES 命中解码为视频结果项，兼容 snake_case 与 camelCase 字段名。
func parseVideoList(res *elastic.SearchResult) []app.SearchVideoHit {
	if res == nil || res.Error != nil || res.Hits == nil {
		return nil
	}

	list := make([]app.SearchVideoHit, 0, len(res.Hits.Hits))

	for _, hit := range res.Hits.Hits {
		var data map[string]any
		if err := json.Unmarshal(hit.Source, &data); err != nil {
			continue
		}

		authorUsername := toString(data["author_username"])
		if authorUsername == "" {
			authorUsername = toString(data["authorUsername"])
		}

		createdAt := toTime(data["created_at"])
		if createdAt.IsZero() {
			createdAt = toTime(data["createdAt"])
		}

		list = append(list, app.SearchVideoHit{
			ID:             toUint(data["id"]),
			Title:          toString(data["title"]),
			Cover:          toString(data["cover"]),
			AuthorUsername: authorUsername,
			CreatedAt:      createdAt,
			Highlight:      toHighlightMap(hit.Highlight),
			Views:          toInt64(data["views"]),
			DanmuCount:     toInt64(data["danmu_count"]),
			Duration:       toInt(data["duration"]),
			Tags:           toStringSlice(data["tags"]),
		})
	}

	return list
}

// parseUserList 把 ES 命中解码为用户结果项。
func parseUserList(res *elastic.SearchResult) []app.SearchUserHit {
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
			ID:            toUint(data["id"]),
			Username:      toString(data["username"]),
			Avatar:        toString(data["avatar"]),
			Highlight:     toHighlightMap(hit.Highlight),
			Level:         toInt(data["level"]),
			FollowerCount: toInt64(data["follower_count"]),
		})
	}

	return list
}

// parseSuggestHits 从命中文档里抽取联想候选值并去重（大小写不敏感）。
//
// ES 拼音命中时原文可能不含查询词，此处仍然保留，由 _score 保证相关性。
func parseSuggestHits(resp *elastic.SearchResult, docType, prefix string, size int) []app.SuggestionItem {
	if resp == nil || resp.Hits == nil || len(resp.Hits.Hits) == 0 {
		return []app.SuggestionItem{}
	}

	items := make([]app.SuggestionItem, 0, size)
	seen := make(map[string]struct{}, size)

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
			Highlight: domain.HighlightSuggestion(value, prefix),
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
		case domain.DocTypeUser:
			if appendValue(toString(data["username"])) {
				return items
			}
		case domain.DocTypeTag:
			for _, tag := range domain.SelectMatchingTags(toStringSlice(data["tags"]), prefix) {
				if appendValue(tag) {
					return items
				}
			}
		default:
			if appendValue(toString(data["title"])) {
				return items
			}
		}
	}

	return items
}

// parseMentionSearchHitUserID 从命中项取用户 ID：优先解析 _id（users:12），其次取 fields.id。
func parseMentionSearchHitUserID(hit *elastic.SearchHit) uint {
	if hit == nil {
		return 0
	}

	if id, ok := domain.ParseDocumentID(hit.Id); ok {
		return id
	}

	return toUint(hit.Fields["id"])
}

// isPublishedVideoSuggestion / suggestionSourceStatus 保留给旧 completion 路径与单测。
func isPublishedVideoSuggestion(source json.RawMessage) bool {
	status, ok := suggestionSourceStatus(source)

	return ok && status == int(enum.VideoStatusPublished)
}

func suggestionSourceStatus(source json.RawMessage) (int, bool) {
	if len(source) == 0 {
		return 0, false
	}

	var payload map[string]any
	if err := json.Unmarshal(source, &payload); err != nil {
		return 0, false
	}

	value, ok := payload["status"]
	if !ok {
		return 0, false
	}

	switch v := value.(type) {
	case float64:
		return int(v), true
	case string:
		status, err := strconv.Atoi(v)

		return status, err == nil
	default:
		return 0, false
	}
}
