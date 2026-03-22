package search

import (
	"encoding/json"
	"reflect"
	"testing"

	searchdomain "f_ccg_video_back_ddd/internal/domain/search"

	"github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

// 验证 ES _source 解码行为。

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

	list := (Adapter{}).parseVideoList(res)
	if len(list) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(list))
	}

	if list[0].AuthorUsername != "alice" {
		t.Fatalf("expected first author alice, got %q", list[0].AuthorUsername)
	}

	if list[0].CreatedAt.IsZero() {
		t.Fatal("expected first createdAt to be parsed")
	}

	if list[1].AuthorUsername != "bob" {
		t.Fatalf("expected second author bob, got %q", list[1].AuthorUsername)
	}

	if list[1].CreatedAt.IsZero() {
		t.Fatal("expected second createdAt to be parsed")
	}
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

	list := (Adapter{}).parseVideoList(res)
	if len(list) != 1 || list[0].ID != 3 {
		t.Fatalf("list = %+v", list)
	}
}

// TestParseVideoHitsReturnsZeroOnShardError 分片级错误时只返回零值（应用层据此回退/告警）。
func TestParseVideoHitsReturnsZeroOnShardError(t *testing.T) {
	t.Parallel()

	if got := (Adapter{}).parseVideoHits(nil, zap.NewNop()); got.Total != 0 || got.Items != nil {
		t.Fatalf("nil response = %+v", got)
	}

	res := &elastic.SearchResult{Error: &elastic.ErrorDetails{Type: "search_phase_execution_exception"}}
	if got := (Adapter{}).parseVideoHits(res, zap.NewNop()); got.Total != 0 || got.Items != nil {
		t.Fatalf("errored response = %+v", got)
	}

	if got := (Adapter{}).parseUserHits(res, zap.NewNop()); got.Total != 0 || got.Items != nil {
		t.Fatalf("errored user response = %+v", got)
	}
}

func TestParseUserListDecodesFields(t *testing.T) {
	t.Parallel()

	res := &elastic.SearchResult{
		Hits: &elastic.SearchHits{
			Hits: []*elastic.SearchHit{
				{
					Source:    json.RawMessage(`{"id":7,"username":"alice","avatar":"a.png","level":3,"follower_count":12}`),
					Highlight: map[string][]string{"username": {"<em>ali</em>ce"}},
				},
			},
		},
	}

	list := (Adapter{}).parseUserList(res)
	if len(list) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(list))
	}

	got := list[0]
	if got.ID != 7 || got.Username != "alice" || got.Avatar != "a.png" || got.Level != 3 || got.FollowerCount != 12 {
		t.Fatalf("hit = %+v", got)
	}

	if !reflect.DeepEqual(got.Highlight["username"], []string{"<em>ali</em>ce"}) {
		t.Fatalf("highlight = %+v", got.Highlight)
	}
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

	items := (Adapter{}).parseSuggestHits(resp, searchdomain.DocTypeVideo, "hell", 10)
	if len(items) != 2 {
		t.Fatalf("items = %+v, want 2 entries", items)
	}

	if items[0].Value != "HelloWorld" || items[0].Highlight != "<em>Hell</em>oWorld" {
		t.Fatalf("first item = %+v", items[0])
	}

	if items[0].DocType != searchdomain.DocTypeVideo {
		t.Fatalf("docType = %q", items[0].DocType)
	}

	if got := (Adapter{}).parseSuggestHits(resp, searchdomain.DocTypeVideo, "hell", 1); len(got) != 1 {
		t.Fatalf("size clamp failed: %+v", got)
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

	items := (Adapter{}).parseSuggestHits(resp, searchdomain.DocTypeTag, "教程", 10)
	if len(items) != 2 || items[0].Value != "教程" || items[1].Value != "Go 教程" {
		t.Fatalf("items = %+v", items)
	}
}

func TestParseSuggestHitsForUsers(t *testing.T) {
	t.Parallel()

	resp := &elastic.SearchResult{
		Hits: &elastic.SearchHits{
			Hits: []*elastic.SearchHit{{Source: json.RawMessage(`{"username":"alice"}`)}},
		},
	}

	items := (Adapter{}).parseSuggestHits(resp, searchdomain.DocTypeUser, "ali", 10)
	if len(items) != 1 || items[0].Value != "alice" || items[0].DocType != searchdomain.DocTypeUser {
		t.Fatalf("items = %+v", items)
	}
}

// TestParseSuggestHitsReturnsEmptySliceForNoHits 零命中返回空切片（不是 nil）。
func TestParseSuggestHitsReturnsEmptySliceForNoHits(t *testing.T) {
	t.Parallel()

	for _, resp := range []*elastic.SearchResult{
		nil,
		{},
		{Hits: &elastic.SearchHits{}},
	} {
		items := (Adapter{}).parseSuggestHits(resp, searchdomain.DocTypeVideo, "x", 10)
		if items == nil || len(items) != 0 {
			t.Fatalf("items = %v, want empty slice", items)
		}
	}
}

func TestParseMentionSearchHitUserID(t *testing.T) {
	t.Parallel()

	if got := (Adapter{}).parseMentionSearchHitUserID(&elastic.SearchHit{Id: "users:56"}); got != 56 {
		t.Fatalf("expected 56, got %d", got)
	}

	if got := (Adapter{}).parseMentionSearchHitUserID(&elastic.SearchHit{Id: "78"}); got != 78 {
		t.Fatalf("expected 78, got %d", got)
	}

	if got := (Adapter{}).parseMentionSearchHitUserID(nil); got != 0 {
		t.Fatalf("nil hit = %d, want 0", got)
	}

	// _id 无法解析时回落到 fields.id。
	hit := &elastic.SearchHit{Id: "users:abc", Fields: map[string]any{"id": float64(91)}}
	if got := (Adapter{}).parseMentionSearchHitUserID(hit); got != 91 {
		t.Fatalf("fields fallback = %d, want 91", got)
	}
}
