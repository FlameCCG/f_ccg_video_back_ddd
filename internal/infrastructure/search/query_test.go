package search

import (
	"encoding/json"
	"f_ccg_video_back_ddd/pkg/constants"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/search"

	domain "f_ccg_video_back_ddd/internal/domain/search"

	"github.com/olivere/elastic/v7"
)

// 迁移自原 service/search_service/search_test.go 的查询构造用例。
// 查询 DSL（字段名、boost 权重、过滤条件、`_source` 白名单）属于检索契约。

// marshalSource 把 SearchSource 序列化成 JSON 字符串，便于断言 DSL 结构。
func marshalSource(t *testing.T, src *elastic.SearchSource) string {
	t.Helper()

	source, err := src.Source()
	require.NoError(t, err, "build search source failed: %v", err)

	raw, err := json.Marshal(source)
	require.NoError(t, err, "marshal search source failed: %v", err)

	return string(raw)
}

func TestBuildVideoSearchSourceFiltersPublished(t *testing.T) {
	t.Parallel()

	payload := marshalSource(t, buildVideoSearchSource(app.Options{
		Keyword:  "教程",
		Page:     1,
		PageSize: 20,
	}, 0))

	require.Contains(t, payload, `"status"`, "expected published status filter in query, got %s", payload)
	require.Contains(t, payload, "1", "expected published status filter in query, got %s", payload)
}

// TestBuildVideoSearchSourceIncludesContractFields 高亮字段与 `_source` 白名单
// 必须与原实现一致，否则响应内容会变。
func TestBuildVideoSearchSourceIncludesContractFields(t *testing.T) {
	t.Parallel()

	payload := marshalSource(t, buildVideoSearchSource(app.Options{
		Keyword:  "教程",
		Page:     3,
		PageSize: 15,
	}, 30))

	for _, field := range videoSourceIncludes {
		require.Contains(t, payload, `"`+field+`"`, "missing _source field %q in %s", field, payload)
	}

	for _, field := range videoHighlightFields {
		require.Contains(t, payload, `"`+field+`"`, "missing highlight field %q in %s", field, payload)
	}

	require.Contains(t, payload, `"from":30`, "paging not applied: %s", payload)
	require.Contains(t, payload, `"size":15`, "paging not applied: %s", payload)

	require.Contains(t, payload, `"track_total_hits":true`, "track_total_hits must stay enabled: %s", payload)

	// olivere/elastic 用 encoding/json 序列化，< > 会被转义成 \u003c / \u003e，
	// 因此按 json.Marshal 的结果比对，而不是写死转义形态。
	preTags, err := json.Marshal([]string{"<em>"})
	require.NoError(t, err, "marshal pre_tags: %v", err)

	postTags, err := json.Marshal([]string{"</em>"})
	require.NoError(t, err, "marshal post_tags: %v", err)

	require.Contains(t, payload, `"pre_tags":`+string(preTags), "highlight tags must stay <em>/</em>: %s", payload)
	require.Contains(t, payload, `"post_tags":`+string(postTags), "highlight tags must stay <em>/</em>: %s", payload)
}

func TestBuildUserSearchSourceIncludesContractFields(t *testing.T) {
	t.Parallel()

	payload := marshalSource(t, buildUserSearchSource(app.Options{
		Keyword:  testAlice,
		Page:     1,
		PageSize: 20,
	}, 0))

	for _, field := range userSourceIncludes {
		require.Contains(t, payload, `"`+field+`"`, "missing _source field %q in %s", field, payload)
	}

	require.Contains(t, payload, `"username.keyword"`, "user query must hit keyword and pinyin sub-fields: %s", payload)
	require.Contains(t, payload, `"username.pinyin"`, "user query must hit keyword and pinyin sub-fields: %s", payload)
}

// TestBuildVideoQueryMatchesAllForEmptyKeyword 空关键词退化为 match_all（浏览态）。
func TestBuildVideoQueryMatchesAllForEmptyKeyword(t *testing.T) {
	t.Parallel()

	for _, build := range []func(string) elastic.Query{buildVideoQuery, buildUserQuery} {
		src, err := build("").Source()
		require.NoError(t, err, "query source: %v", err)

		raw, err := json.Marshal(src)
		require.NoError(t, err, "marshal: %v", err)

		require.Contains(t, string(raw), "match_all", "empty keyword should build match_all, got %s", raw)
	}
}

// TestBuildVideoQueryEscapesWildcards 关键词含通配特殊字符时必须转义后再拼进
// wildcard 查询，避免用户输入被当成通配模式。
func TestBuildVideoQueryEscapesWildcards(t *testing.T) {
	t.Parallel()

	src, err := buildVideoQuery("ce6*").Source()
	require.NoError(t, err, "query source: %v", err)

	raw, err := json.Marshal(src)
	require.NoError(t, err, "marshal: %v", err)

	require.Contains(t, string(raw), `ce6\\*`, "wildcard must be escaped, got %s", raw)
}

// TestApplyVideoSortToSourceAlwaysFallsBackToScore 非相关性排序在业务字段之后
// 追加 _score 兜底；相关性排序只按 _score 倒序。
func TestApplyVideoSortToSourceAlwaysFallsBackToScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field int
		order int
		want  string
	}{
		{name: "views desc", field: domain.VideoSortViews, order: domain.SortOrderDesc, want: `"views"`},
		{name: "danmu asc", field: domain.VideoSortDanmu, order: domain.SortOrderAsc, want: `"danmu_count"`},
		{name: "duration desc", field: domain.VideoSortDuration, order: domain.SortOrderDesc, want: `"duration"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			source := elastic.NewSearchSource().Query(elastic.NewMatchAllQuery())
			applyVideoSortToSource(source, tt.field, tt.order)

			payload := marshalSource(t, source)
			require.Contains(t, payload, tt.want, "missing sort field %s in %s", tt.want, payload)

			require.Contains(t, payload, `"_score"`, "_score tiebreaker missing in %s", payload)
		})
	}

	relevance := elastic.NewSearchSource().Query(elastic.NewMatchAllQuery())
	applyVideoSortToSource(relevance, domain.VideoSortRelevance, domain.SortOrderDesc)

	payload := marshalSource(t, relevance)
	require.NotContains(t, payload, `"views"`, "relevance sort must not add business fields: %s", payload)
}

func TestApplyUserSortToSourceAlwaysFallsBackToScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field int
		want  string
	}{
		{name: "followers", field: domain.UserSortFollowers, want: `"follower_count"`},
		{name: "level", field: domain.UserSortLevel, want: `"level"`},
		{name: "created at", field: domain.UserSortCreatedAt, want: `"created_at"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			source := elastic.NewSearchSource().Query(elastic.NewMatchAllQuery())
			applyUserSortToSource(source, tt.field, domain.SortOrderDesc)

			payload := marshalSource(t, source)
			require.Contains(t, payload, tt.want, "sort payload = %s", payload)
			require.Contains(t, payload, `"_score"`, "sort payload = %s", payload)
		})
	}
}

// TestSuggestIndexAndFilterRouting 联想维度决定命中索引与是否只取已发布视频。
func TestSuggestIndexAndFilterRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		docType       string
		wantIndex     string
		publishedOnly bool
		wantField     string
	}{
		{
			docType:       domain.DocTypeVideo,
			wantIndex:     constants.VideoSearchIndex,
			publishedOnly: true,
			wantField:     "title",
		},
		{
			docType:       domain.DocTypeTag,
			wantIndex:     constants.VideoSearchIndex,
			publishedOnly: true,
			wantField:     "tags",
		},
		{
			docType:       domain.DocTypeUser,
			wantIndex:     constants.UserSearchIndex,
			publishedOnly: false,
			wantField:     fieldUsername,
		},
	}

	for _, tt := range tests {
		t.Run(tt.docType, func(t *testing.T) {
			t.Parallel()

			{
				got := suggestIndexFor(tt.docType)
				require.Equal(t, tt.wantIndex, got, "index = %q, want %q", got, tt.wantIndex)
			}

			{
				got := suggestPublishedOnly(tt.docType)
				require.Equal(t, got, tt.publishedOnly, "publishedOnly = %v, want %v", got, tt.publishedOnly)
			}

			fields := suggestSourceFields(tt.docType)
			require.Len(t, fields, 1, "_source fields = %v, want [%s]", fields, tt.wantField)
			require.Equal(t, tt.wantField, fields[0], "_source fields = %v, want [%s]", fields, tt.wantField)

			src, err := buildSuggestQuery(tt.docType, "ce6").Source()
			require.NoError(t, err, "suggest query source: %v", err)

			raw, err := json.Marshal(src)
			require.NoError(t, err, "marshal: %v", err)

			require.Contains(t, string(raw), tt.wantField+".keyword", "suggest query must hit %s.keyword, got %s", tt.wantField, raw)
		})
	}
}

// TestBuildSuggestFieldQueryRejectsBlankKeyword 空白关键词构造 match_none，
// 避免把整索引召回。
func TestBuildSuggestFieldQueryRejectsBlankKeyword(t *testing.T) {
	t.Parallel()

	src, err := buildSuggestFieldQuery("   ", "title", "title.keyword", "title.pinyin").Source()
	require.NoError(t, err, "query source: %v", err)

	raw, err := json.Marshal(src)
	require.NoError(t, err, "marshal: %v", err)

	require.Contains(t, string(raw), "match_none", "blank keyword should build match_none, got %s", raw)
}

// TestBuildSearchRequestsCarryIndexAndSort 综合搜索的两个子请求都带上目标索引，
// 且各自施加了对应维度的排序（视频侧另有「仅已发布」过滤）。
//
// olivere/elastic 的请求头（index）是非导出字段，无法直接断言索引名；
// 索引常量本身由 pkg/constants 固定，此处只校验请求确实指定了索引并携带正确 DSL。
func TestBuildSearchRequestsCarryIndexAndSort(t *testing.T) {
	t.Parallel()

	opts := app.Options{
		Keyword:   "教程",
		Page:      1,
		PageSize:  20,
		VideoSort: domain.VideoSortViews,
		UserSort:  domain.UserSortFollowers,
	}

	videoReq := buildVideoSearchRequest(opts, 0)
	require.True(t, videoReq.HasIndices(), "video request must target an index")

	videoBody, err := videoReq.Body()
	require.NoError(t, err, "video request body: %v", err)

	require.Contains(t, videoBody, `"views"`, "video request body = %s", videoBody)
	require.Contains(t, videoBody, `"status"`, "video request body = %s", videoBody)

	userReq := buildUserSearchRequest(opts, 0)
	require.True(t, userReq.HasIndices(), "user request must target an index")

	userBody, err := userReq.Body()
	require.NoError(t, err, "user request body: %v", err)

	require.Contains(t, userBody, `"follower_count"`, "user request body = %s", userBody)

	// 用户索引不做「仅已发布」过滤（用户没有发布状态）。
	require.NotContains(t, userBody, `"status"`, "user request must not filter by video status: %s", userBody)
}
