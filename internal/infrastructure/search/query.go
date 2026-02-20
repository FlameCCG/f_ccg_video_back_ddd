package search

import (
	app "f_ccg_video_back_ddd/internal/application/search"
	domain "f_ccg_video_back_ddd/internal/domain/search"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/olivere/elastic/v7"
)

// 各索引使用的字段集合（只读常量表，构建查询时按需引用）。
// 字段名、boost 权重与 `_source` 白名单都属于检索契约，改动会直接改变命中与响应内容。
var (
	videoHighlightFields = []string{"title", "tags"}
	userHighlightFields  = []string{"username"}

	videoSourceIncludes = []string{
		"id", "title", "cover", "author_username", "created_at",
		"views", "danmu_count", "duration", "tags",
	}
	userSourceIncludes = []string{"id", "username", "avatar", "level", "follower_count"}
)

func buildVideoQuery(keyword string) elastic.Query {
	if keyword == "" {
		return elastic.NewMatchAllQuery()
	}

	lower := domain.LowerKeyword(keyword)
	should := []elastic.Query{
		elastic.NewMatchQuery("title", keyword).Boost(5).MinimumShouldMatch("80%"),
		elastic.NewMatchQuery("title.pinyin", keyword).Boost(3),
		elastic.NewMatchQuery("tags", keyword).Boost(4),
		elastic.NewMatchQuery("tags.pinyin", keyword).Boost(3),
		elastic.NewPrefixQuery("title.keyword", lower).Boost(4),
		elastic.NewPrefixQuery("tags.keyword", lower).Boost(3),
	}

	// 中间片段匹配（如标题/标签内嵌 id 段），复用 keyword 倒排，不新增 ngram 字段。
	if escaped, ok := domain.EscapeESWildcard(lower); ok {
		should = append(should,
			elastic.NewWildcardQuery("title.keyword", "*"+escaped+"*").Boost(2),
			elastic.NewWildcardQuery("tags.keyword", "*"+escaped+"*").Boost(2),
		)
	}

	return elastic.NewBoolQuery().Should(should...).MinimumNumberShouldMatch(1)
}

func buildUserQuery(keyword string) elastic.Query {
	if keyword == "" {
		return elastic.NewMatchAllQuery()
	}

	lower := domain.LowerKeyword(keyword)
	should := []elastic.Query{
		elastic.NewTermQuery("username.keyword", lower).Boost(10),
		elastic.NewPrefixQuery("username.keyword", lower).Boost(8),
		elastic.NewMatchQuery("username", keyword).Boost(6),
		elastic.NewMatchQuery("username.pinyin", keyword).Boost(5),
	}

	// 支持 c / ce / ce6 命中 毛民_ce6ffa… 这类中间片段。
	if escaped, ok := domain.EscapeESWildcard(lower); ok {
		should = append(should, elastic.NewWildcardQuery("username.keyword", "*"+escaped+"*").Boost(4))
	}

	return elastic.NewBoolQuery().Should(should...).MinimumNumberShouldMatch(1)
}

func buildPublishedVideoQuery(keyword string) elastic.Query {
	return elastic.NewBoolQuery().
		Must(buildVideoQuery(keyword)).
		Filter(elastic.NewTermQuery("status", int(enum.VideoStatusPublished)))
}

func buildHighlight(fields []string) *elastic.Highlight {
	highlight := elastic.NewHighlight().PreTags("<em>").PostTags("</em>").FragmentSize(150).NumOfFragments(1)
	for _, field := range fields {
		highlight = highlight.Field(field)
	}

	return highlight
}

func applyVideoSortToSource(source *elastic.SearchSource, sortField, sortOrder int) {
	ascending := sortOrder == domain.SortOrderAsc

	switch sortField {
	case domain.VideoSortViews:
		// 非相关性排序时，先按业务字段排，再按 _score 兜底，保证同值时更相关的内容优先。
		source.Sort("views", ascending)
	case domain.VideoSortDanmu:
		source.Sort("danmu_count", ascending)
	case domain.VideoSortDuration:
		source.Sort("duration", ascending)
	default:
		// 相关性排序直接按 ES 默认 _score 倒序。
		source.Sort("_score", false)

		return
	}

	source.Sort("_score", false)
}

func applyUserSortToSource(source *elastic.SearchSource, sortField, sortOrder int) {
	ascending := sortOrder == domain.SortOrderAsc

	switch sortField {
	case domain.UserSortFollowers:
		// 非相关性排序时，先按业务字段排，再按 _score 兜底，保证同值时更相关的用户优先。
		source.Sort("follower_count", ascending)
	case domain.UserSortLevel:
		source.Sort("level", ascending)
	case domain.UserSortCreatedAt:
		source.Sort("created_at", ascending)
	default:
		// 相关性排序直接按 ES 默认 _score 倒序。
		source.Sort("_score", false)

		return
	}

	source.Sort("_score", false)
}

func buildVideoSearchSource(opts app.SearchOptions, from int) *elastic.SearchSource {
	return elastic.NewSearchSource().
		Query(buildPublishedVideoQuery(opts.Keyword)).
		Highlight(buildHighlight(videoHighlightFields)).
		FetchSourceContext(elastic.NewFetchSourceContext(true).Include(videoSourceIncludes...)).
		From(from).
		Size(opts.PageSize).
		TrackTotalHits(true)
}

func buildUserSearchSource(opts app.SearchOptions, from int) *elastic.SearchSource {
	return elastic.NewSearchSource().
		Query(buildUserQuery(opts.Keyword)).
		Highlight(buildHighlight(userHighlightFields)).
		FetchSourceContext(elastic.NewFetchSourceContext(true).Include(userSourceIncludes...)).
		From(from).
		Size(opts.PageSize).
		TrackTotalHits(true)
}

func buildVideoSearchRequest(opts app.SearchOptions, from int) *elastic.SearchRequest {
	source := buildVideoSearchSource(opts, from)
	applyVideoSortToSource(source, opts.VideoSort, opts.VideoOrder)

	return elastic.NewSearchRequest().
		Index(constants.VideoSearchIndex).
		SearchSource(source)
}

func buildUserSearchRequest(opts app.SearchOptions, from int) *elastic.SearchRequest {
	source := buildUserSearchSource(opts, from)
	applyUserSortToSource(source, opts.UserSort, opts.UserOrder)

	return elastic.NewSearchRequest().
		Index(constants.UserSearchIndex).
		SearchSource(source)
}

// suggestIndexFor 返回联想查询命中的索引名。
func suggestIndexFor(docType string) string {
	if docType == domain.DocTypeUser {
		return constants.UserSearchIndex
	}

	return constants.VideoSearchIndex
}

// suggestPublishedOnly 报告该联想维度是否只取已发布视频（用户维度不过滤）。
func suggestPublishedOnly(docType string) bool {
	return docType != domain.DocTypeUser
}

func suggestSourceFields(docType string) []string {
	switch docType {
	case domain.DocTypeUser:
		return []string{"username"}
	case domain.DocTypeTag:
		return []string{"tags"}
	default:
		return []string{"title"}
	}
}

// buildSuggestQuery 复用已有字段：精确/前缀/正文/拼音/通配中间匹配。
// 通配仅在 keyword 上做，避免新增 ngram 字段带来的空间膨胀。
func buildSuggestQuery(docType, keyword string) elastic.Query {
	switch docType {
	case domain.DocTypeUser:
		return buildSuggestFieldQuery(keyword, "username", "username.keyword", "username.pinyin")
	case domain.DocTypeTag:
		return buildSuggestFieldQuery(keyword, "tags", "tags.keyword", "tags.pinyin")
	default:
		return buildSuggestFieldQuery(keyword, "title", "title.keyword", "title.pinyin")
	}
}

func buildSuggestFieldQuery(keyword, textField, keywordField, pinyinField string) elastic.Query {
	lower := domain.LowerKeyword(keyword)
	if lower == "" {
		return elastic.NewMatchNoneQuery()
	}

	should := []elastic.Query{
		elastic.NewTermQuery(keywordField, lower).Boost(12),
		elastic.NewPrefixQuery(keywordField, lower).Boost(10),
		elastic.NewMatchPhrasePrefixQuery(textField, keyword).Boost(8),
		elastic.NewMatchQuery(pinyinField, keyword).Boost(7),
		elastic.NewMatchQuery(textField, keyword).Boost(5),
	}

	// 中间片段：毛民_ce6ffa… 用 c/ce/ce6 命中。短查询也允许，靠 _score + size 控制噪音。
	if escaped, ok := domain.EscapeESWildcard(lower); ok {
		should = append(should, elastic.NewWildcardQuery(keywordField, "*"+escaped+"*").Boost(4))
	}

	return elastic.NewBoolQuery().Should(should...).MinimumNumberShouldMatch(1)
}
