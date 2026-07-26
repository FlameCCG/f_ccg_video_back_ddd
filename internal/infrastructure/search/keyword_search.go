package search

import (
	"context"

	app "f_ccg_video_back_ddd/internal/application/search"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

// suggestCandidateLimit 联想过采样上限：过采样以便去重后仍够 size，同时控制查询成本.
const suggestCandidateLimit = 40

// Adapter 是搜索上下文的 Elasticsearch v7 检索适配器。
//
// 同时实现关键词检索、联想与 @ 提及匹配三个应用层端口；
// 索引名、查询 DSL、boost 权重、`_source` 白名单全部与原 service/search_service 逐字一致。
// client 为 nil 时 Available() 返回 false，应用层据此走 MySQL 回退（等价原 global.ESClient == nil 判断）。
type Adapter struct {
	client *elastic.Client
}

var (
	_ app.KeywordSearch = (*Adapter)(nil)
	_ app.SuggestSearch = (*Adapter)(nil)
	_ app.MentionSearch = (*Adapter)(nil)
)

// NewAdapter 构造 Elasticsearch 检索适配器（client 允许为 nil）。
func NewAdapter(client *elastic.Client) *Adapter {
	return &Adapter{client: client}
}

// Available 报告 ES 客户端是否就绪。
func (a *Adapter) Available() bool {
	return a != nil && a.client != nil
}

// MultiSearch 一次请求同时检索视频与用户索引。
func (a *Adapter) MultiSearch(ctx context.Context, opts app.SearchOptions, from int) (app.CombinedHits, error) {
	if !a.Available() {
		return app.CombinedHits{}, app.ErrESUnavailable
	}

	resp, err := a.client.MultiSearch().
		Add(buildVideoSearchRequest(opts, from)).
		Add(buildUserSearchRequest(opts, from)).
		Do(ctx)
	if err != nil {
		return app.CombinedHits{}, err
	}

	if len(resp.Responses) != 2 {
		return app.CombinedHits{}, app.ErrUnexpectedMultiSearchSize
	}

	return app.CombinedHits{
		Videos: parseVideoHits(resp.Responses[0]),
		Users:  parseUserHits(resp.Responses[1]),
	}, nil
}

// SearchVideos 检索视频索引（已发布过滤 + 排序 + 高亮）。
func (a *Adapter) SearchVideos(ctx context.Context, opts app.SearchOptions, from int) (app.VideoHits, error) {
	if !a.Available() {
		return app.VideoHits{}, app.ErrESUnavailable
	}

	source := buildVideoSearchSource(opts, from)
	applyVideoSortToSource(source, opts.VideoSort, opts.VideoOrder)

	resp, err := a.client.Search().
		Index(constants.VideoSearchIndex).
		SearchSource(source).
		Do(ctx)
	if err != nil {
		return app.VideoHits{}, err
	}

	return app.VideoHits{Total: resp.TotalHits(), Items: parseVideoList(resp)}, nil
}

// SearchUsers 检索用户索引（排序 + 高亮）。
func (a *Adapter) SearchUsers(ctx context.Context, opts app.SearchOptions, from int) (app.UserHits, error) {
	if !a.Available() {
		return app.UserHits{}, app.ErrESUnavailable
	}

	source := buildUserSearchSource(opts, from)
	applyUserSortToSource(source, opts.UserSort, opts.UserOrder)

	resp, err := a.client.Search().
		Index(constants.UserSearchIndex).
		SearchSource(source).
		Do(ctx)
	if err != nil {
		return app.UserHits{}, err
	}

	return app.UserHits{Total: resp.TotalHits(), Items: parseUserList(resp)}, nil
}

// CountUsers 只取用户索引命中总量（size=0）。
func (a *Adapter) CountUsers(ctx context.Context, keyword string) (int64, error) {
	if !a.Available() {
		return 0, app.ErrESUnavailable
	}

	resp, err := a.client.Search().
		Index(constants.UserSearchIndex).
		SearchSource(elastic.NewSearchSource().
			Query(buildUserQuery(keyword)).
			Size(0).
			TrackTotalHits(true)).
		Do(ctx)
	if err != nil {
		return 0, err
	}

	return resp.TotalHits(), nil
}

// FieldSuggest 基于 text/keyword/pinyin 倒排索引取联想候选。
func (a *Adapter) FieldSuggest(
	ctx context.Context,
	docType, prefix string,
	size int,
) ([]app.SuggestionItem, error) {
	if !a.Available() {
		return nil, app.ErrESUnavailable
	}

	if size <= 0 {
		return []app.SuggestionItem{}, nil
	}

	// 过采样以便去重后仍够 size；上限控制查询成本。
	candidateSize := min(max(size*4, size), suggestCandidateLimit)

	query := buildSuggestQuery(docType, prefix)
	if suggestPublishedOnly(docType) {
		query = elastic.NewBoolQuery().
			Must(query).
			Filter(elastic.NewTermQuery("status", int(enum.VideoStatusPublished)))
	}

	source := elastic.NewSearchSource().
		Query(query).
		FetchSourceContext(elastic.NewFetchSourceContext(true).Include(suggestSourceFields(docType)...)).
		Size(candidateSize).
		Sort("_score", false).
		TrackTotalHits(false)

	index := suggestIndexFor(docType)

	resp, err := elastic.NewSearchService(a.client).
		Index(index).
		SearchSource(source).
		Do(ctx)
	if err != nil {
		zap.L().Warn("suggest search failed",
			zap.String("index", index), zap.String("docType", docType), zap.Error(err))

		return nil, err
	}

	return parseSuggestHits(resp, docType, prefix, size), nil
}

// MatchUserIDs 在候选用户 ID 集合内做用户名/拼音匹配，返回命中的用户 ID（ES 相关性顺序）。
func (a *Adapter) MatchUserIDs(ctx context.Context, userIDs []uint, keyword string) ([]uint, error) {
	if !a.Available() {
		return nil, app.ErrESUnavailable
	}

	if len(userIDs) == 0 {
		return []uint{}, nil
	}

	ids := make([]any, 0, len(userIDs))
	for _, id := range userIDs {
		ids = append(ids, int64(id))
	}

	source := elastic.NewSearchSource().
		Query(elastic.NewBoolQuery().
			Filter(elastic.NewTermsQuery("id", ids...)).
			Must(buildUserQuery(keyword)),
		).
		FetchSource(false).
		Size(len(userIDs)).
		TrackTotalHits(true)

	resp, err := a.client.Search().
		Index(constants.UserSearchIndex).
		SearchSource(source).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	if resp == nil || resp.Hits == nil {
		return []uint{}, nil
	}

	matched := make([]uint, 0, len(resp.Hits.Hits))

	for _, hit := range resp.Hits.Hits {
		if id := parseMentionSearchHitUserID(hit); id != 0 {
			matched = append(matched, id)
		}
	}

	return matched, nil
}
