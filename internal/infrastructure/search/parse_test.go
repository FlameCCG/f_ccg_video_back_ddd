package search

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/search"

	"github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

// 迁移自原 service/search_service/search_test.go 与 suggest_test.go 中
// 与 ES `_source` 解码相关的用例。

func TestParseVideoListSupportsSnakeAndCamelFields(t *testing.T) {
	t.Parallel()

	res := &elastic.SearchResult{
		Hits: &elastic.SearchHits{
			Hits: []*elastic.SearchHit{
				{
					Source: json.RawMessage(`{
						"id": 1,
						"title": "视频1",
						"cover": "cover-1",
						"author_username": "alice",
						"created_at": "2026-03-24T10:20:30Z",
						"views": 100,
						"danmu_count": 3,
						"duration": 60,
						"tags": ["教程"]
					}`),
				},
				{
					Source: json.RawMessage(`{
						"id": 2,
						"title": "视频2",
						"cover": "cover-2",
						"authorUsername": "bob",
						"createdAt": "2026-03-24 10:20:30",
						"views": 200,
						"danmu_count": 5,
						"duration": 120,
						"tags": ["Vlog"]
					}`),
				},
			},
		},
	}

	list := parseVideoList(res)
	require.Len(t, list, 2, "expected 2 hits, got %d", len(list))

	require.Equal(t, testAlice, list[0].AuthorUsername, "expected first author alice, got %q", list[0].AuthorUsername)

	require.False(t, list[0].CreatedAt.IsZero(), "expected first createdAt to be parsed")

	require.Equal(t, "bob", list[1].AuthorUsername, "expected second author bob, got %q", list[1].AuthorUsername)

	require.False(t, list[1].CreatedAt.IsZero(), "expected second createdAt to be parsed")
}

// TestParseVideoListSkipsUndecodableHits 单条文档解码失败只跳过该条，不影响整页。
func TestParseVideoListSkipsUndecodableHits(t *testing.T) {
	t.Parallel()

	res := &elastic.SearchResult{
		Hits: &elastic.SearchHits{
			Hits: []*elastic.SearchHit{
				{Source: json.RawMessage(`{ not json`)},
				{Source: json.RawMessage(`{"id": 3, "title": "ok"}`)},
			},
		},
	}

	list := parseVideoList(res)
	require.Len(t, list, 1, "list = %+v", list)
	require.EqualValues(t, 3, list[0].ID, "list = %+v", list)
}

// TestParseVideoHitsReturnsZeroOnShardError 分片级错误时只返回零值（应用层据此回退/告警）。
func TestParseVideoHitsReturnsZeroOnShardError(t *testing.T) {
	t.Parallel()

	{
		got := parseVideoHits(nil, zap.NewNop())
		require.EqualValues(t, 0, got.Total, "nil response = %+v", got)
		require.Nil(t, got.Items, "nil response = %+v", got)
	}

	res := &elastic.SearchResult{Error: &elastic.ErrorDetails{Type: "search_phase_execution_exception"}}
	{
		got := parseVideoHits(res, zap.NewNop())
		require.EqualValues(t, 0, got.Total, "errored response = %+v", got)
		require.Nil(t, got.Items, "errored response = %+v", got)
	}

	{
		got := parseUserHits(res, zap.NewNop())
		require.EqualValues(t, 0, got.Total, "errored user response = %+v", got)
		require.Nil(t, got.Items, "errored user response = %+v", got)
	}
}

func TestParseUserListDecodesFields(t *testing.T) {
	t.Parallel()

	res := &elastic.SearchResult{
		Hits: &elastic.SearchHits{
			Hits: []*elastic.SearchHit{
				{
					Source:    json.RawMessage(`{"id":7,"username":"alice","avatar":"a.png","level":3,"follower_count":12}`),
					Highlight: map[string][]string{fieldUsername: {"<em>ali</em>ce"}},
				},
			},
		},
	}

	list := parseUserList(res)
	require.Len(t, list, 1, "expected 1 hit, got %d", len(list))

	got := list[0]
	require.EqualValues(t, 7, got.ID, "hit = %+v", got)
	require.Equal(t, testAlice, got.Username, "hit = %+v", got)
	require.Equal(t, "a.png", got.Avatar, "hit = %+v", got)
	require.Equal(t, 3, got.Level, "hit = %+v", got)
	require.EqualValues(t, 12, got.FollowerCount, "hit = %+v", got)

	require.True(t, reflect.DeepEqual(got.Highlight[fieldUsername], []string{"<em>ali</em>ce"}), "highlight = %+v", got.Highlight)
}

// TestParseSuggestHitsDeduplicatesCaseInsensitively 候选值按小写去重，空值丢弃，
// 并按 size 截断。
func TestParseSuggestHitsDeduplicatesCaseInsensitively(t *testing.T) {
	t.Parallel()

	resp := &elastic.SearchResult{
		Hits: &elastic.SearchHits{
			Hits: []*elastic.SearchHit{
				{Source: json.RawMessage(`{"title":"HelloWorld"}`)},
				{Source: json.RawMessage(`{"title":"helloworld"}`)},
				{Source: json.RawMessage(`{"title":"  "}`)},
				{Source: json.RawMessage(`{"title":"Another"}`)},
			},
		},
	}

	items := parseSuggestHits(resp, domain.DocTypeVideo, "hell", 10)
	require.Len(t, items, 2, "items = %+v, want 2 entries", items)

	require.Equal(t, "HelloWorld", items[0].Value, "first item = %+v", items[0])
	require.Equal(t, "<em>Hell</em>oWorld", items[0].Highlight, "first item = %+v", items[0])

	require.Equal(t, domain.DocTypeVideo, items[0].DocType, "docType = %q", items[0].DocType)

	{
		got := parseSuggestHits(resp, domain.DocTypeVideo, "hell", 1)
		require.Len(t, got, 1, "size clamp failed: %+v", got)
	}
}

// TestParseSuggestHitsForTagsPrefersLiteralMatches 标签维度只取与查询词字面相关的标签。
func TestParseSuggestHitsForTagsPrefersLiteralMatches(t *testing.T) {
	t.Parallel()

	resp := &elastic.SearchResult{
		Hits: &elastic.SearchHits{
			Hits: []*elastic.SearchHit{
				{Source: json.RawMessage(`{"tags":["教程","vlog","Go 教程"]}`)},
			},
		},
	}

	items := parseSuggestHits(resp, domain.DocTypeTag, "教程", 10)
	require.Len(t, items, 2, "items = %+v", items)
	require.Equal(t, "教程", items[0].Value, "items = %+v", items)
	require.Equal(t, "Go 教程", items[1].Value, "items = %+v", items)
}

func TestParseSuggestHitsForUsers(t *testing.T) {
	t.Parallel()

	resp := &elastic.SearchResult{
		Hits: &elastic.SearchHits{
			Hits: []*elastic.SearchHit{{Source: json.RawMessage(`{"username":"alice"}`)}},
		},
	}

	items := parseSuggestHits(resp, domain.DocTypeUser, "ali", 10)
	require.Len(t, items, 1, "items = %+v", items)
	require.Equal(t, testAlice, items[0].Value, "items = %+v", items)
	require.Equal(t, domain.DocTypeUser, items[0].DocType, "items = %+v", items)
}

// TestParseSuggestHitsReturnsEmptySliceForNoHits 零命中返回空切片（不是 nil）。
func TestParseSuggestHitsReturnsEmptySliceForNoHits(t *testing.T) {
	t.Parallel()

	for _, resp := range []*elastic.SearchResult{
		nil,
		{},
		{Hits: &elastic.SearchHits{}},
	} {
		items := parseSuggestHits(resp, domain.DocTypeVideo, "x", 10)
		require.NotNil(t, items, "items = %v, want empty slice", items)
		require.Empty(t, items, "items = %v, want empty slice", items)
	}
}

func TestParseMentionSearchHitUserID(t *testing.T) {
	t.Parallel()

	{
		got := parseMentionSearchHitUserID(&elastic.SearchHit{Id: "users:56"})
		require.EqualValues(t, 56, got, "expected 56, got %d", got)
	}

	{
		got := parseMentionSearchHitUserID(&elastic.SearchHit{Id: "78"})
		require.EqualValues(t, 78, got, "expected 78, got %d", got)
	}

	{
		got := parseMentionSearchHitUserID(nil)
		require.EqualValues(t, 0, got, "nil hit = %d, want 0", got)
	}

	// _id 无法解析时回落到 fields.id。
	hit := &elastic.SearchHit{Id: "users:abc", Fields: map[string]any{"id": float64(91)}}
	{
		got := parseMentionSearchHitUserID(hit)
		require.EqualValues(t, 91, got, "fields fallback = %d, want 91", got)
	}
}

func TestIsPublishedVideoSuggestionRequiresPublishedStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "published numeric status", raw: `{"status":1}`, want: true},
		{name: "published string status", raw: `{"status":"1"}`, want: true},
		{name: "private status", raw: `{"status":2}`, want: false},
		{name: "review status", raw: `{"status":4}`, want: false},
		{name: "missing status", raw: `{"title":"demo"}`, want: false},
		{name: "empty source", raw: ``, want: false},
		{name: "malformed json", raw: `{`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			{
				got := isPublishedVideoSuggestion(json.RawMessage(tt.raw))
				require.Equal(t, tt.want, got, "expected %v, got %v", tt.want, got)
			}
		})
	}
}
