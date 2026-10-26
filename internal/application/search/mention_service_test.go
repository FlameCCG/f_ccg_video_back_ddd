package search

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/search"

	"go.uber.org/zap"
)

// 迁移自原 service/search_service/mention_test.go。
//
// 原 TestParseMentionDocID / TestParseMentionSearchHitUserID 依赖 elastic.SearchHit，
// 属于 ES 解码细节，已随实现下移：文档 _id 解析规则落在
// internal/domain/search（ParseDocumentID，见 keyword_test.go），
// 命中项字段提取落在 internal/infrastructure/search（parseMentionSearchHitUserID）。

func TestSortMentionUsers(t *testing.T) {
	t.Parallel()

	users := []MentionUser{
		{ID: 3, Username: "c", FollowerCount: 8, IsFriend: false},
		{ID: 2, Username: "b", FollowerCount: 5, IsFriend: true},
		{ID: 1, Username: "a", FollowerCount: 9, IsFriend: true},
		{ID: 4, Username: "d", FollowerCount: 8, IsFriend: false},
	}

	sortMentionUsers(users)

	got := []uint{users[0].ID, users[1].ID, users[2].ID, users[3].ID}

	want := []uint{1, 2, 3, 4}
	require.True(t, reflect.DeepEqual(got, want), "sorted ids = %v, want %v", got, want)
}

func TestPaginateMentionUsers(t *testing.T) {
	t.Parallel()

	users := []MentionUser{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}

	page := paginateMentionUsers(users, mentionPage(2, 2))
	got := []uint{page[0].ID, page[1].ID}

	want := []uint{3, 4}
	require.True(t, reflect.DeepEqual(got, want), "page ids = %v, want %v", got, want)

	// 越界页返回空切片（而非 nil），保证响应里是 [] 。
	{
		out := paginateMentionUsers(users, mentionPage(9, 2))
		require.NotNil(t, out, "out-of-range page = %v", out)
		require.Empty(t, out, "out-of-range page = %v", out)
	}
}

// TestFilterByKeywordFallsBackToLocalMatch ES 不可用时退化为本地字面匹配。
func TestFilterByKeywordFallsBackToLocalMatch(t *testing.T) {
	t.Parallel()

	users := []MentionUser{
		{ID: 1, Username: testAlice},
		{ID: 2, Username: "Bob"},
		{ID: 3, Username: "Alicia"},
	}
	svc := NewMentionService(&fakeMentionRelations{users: users}, &fakeMentionSearch{available: false}, zap.NewNop())

	got, err := svc.filterByKeyword(context.Background(), users, "ali")
	require.NoError(t, err, "filterByKeyword returned error: %v", err)

	{
		ids := mentionIDs(got)
		require.True(t, reflect.DeepEqual(ids, []uint{1, 3}), "matched ids = %v, want [1 3]", ids)
	}
}

// TestFilterByKeywordIgnoresESErrorAndKeepsLocalMatches ES 报错只记日志，
// 结果仍由本地字面匹配兜底（与原 mention suggest fallback to local match 一致）。
func TestFilterByKeywordIgnoresESError(t *testing.T) {
	t.Parallel()

	users := []MentionUser{
		{ID: 1, Username: testAlice},
		{ID: 2, Username: "Bob"},
	}
	index := &fakeMentionSearch{available: true, err: errors.New("es down")}
	svc := NewMentionService(&fakeMentionRelations{users: users}, index, zap.NewNop())

	got, err := svc.filterByKeyword(context.Background(), users, "ali")
	require.NoError(t, err, "filterByKeyword returned error: %v", err)

	{
		ids := mentionIDs(got)
		require.True(t, reflect.DeepEqual(ids, []uint{1}), "matched ids = %v, want [1]", ids)
	}
}

// TestFilterByKeywordMergesESHits ES 拼音命中（本地字面不含关键词）也要进入结果集。
func TestFilterByKeywordMergesESHits(t *testing.T) {
	t.Parallel()

	users := []MentionUser{
		{ID: 1, Username: "毛民", FollowerCount: 3},
		{ID: 2, Username: "Bob"},
	}
	// "mao" 字面匹配不到「毛民」，只能靠 ES 拼音命中。
	index := &fakeMentionSearch{available: true, ids: []uint{1}}
	svc := NewMentionService(&fakeMentionRelations{users: users}, index, zap.NewNop())

	got, err := svc.filterByKeyword(context.Background(), users, "mao")
	require.NoError(t, err, "filterByKeyword returned error: %v", err)

	{
		ids := mentionIDs(got)
		require.True(t, reflect.DeepEqual(ids, []uint{1}), "matched ids = %v, want [1]", ids)
	}
}

func TestSuggestFollowUsersRejectsAnonymous(t *testing.T) {
	t.Parallel()

	svc := NewMentionService(&fakeMentionRelations{}, &fakeMentionSearch{}, zap.NewNop())

	{
		_, _, err := svc.SuggestFollowUsers(context.Background(), 0, "", 1, 20)
		require.ErrorIs(t, err, ErrInvalidMentionUserID, "err = %v, want ErrInvalidMentionUserID", err)
	}
}

// TestSuggestFollowUsersWithoutKeywordReturnsWholeFollowList 空关键词直接返回全部关注（排序 + 分页）。
func TestSuggestFollowUsersWithoutKeywordReturnsWholeFollowList(t *testing.T) {
	t.Parallel()

	relations := &fakeMentionRelations{users: []MentionUser{
		{ID: 3, FollowerCount: 1},
		{ID: 1, FollowerCount: 9, IsFriend: true},
		{ID: 2, FollowerCount: 5},
	}}
	svc := NewMentionService(relations, &fakeMentionSearch{available: false}, zap.NewNop())

	list, total, err := svc.SuggestFollowUsers(context.Background(), 7, "  ", 1, 2)
	require.NoError(t, err, "SuggestFollowUsers: %v", err)

	require.EqualValues(t, 3, total, "total = %d, want 3", total)

	{
		ids := mentionIDs(list)
		require.True(t, reflect.DeepEqual(ids, []uint{1, 2}), "page ids = %v, want [1 2]", ids)
	}
}

// TestSuggestFollowUsersWithoutFollowingReturnsEmpty 没有关注任何人时返回空列表与 0，
// 不进入关键词过滤路径。
func TestSuggestFollowUsersWithoutFollowingReturnsEmpty(t *testing.T) {
	t.Parallel()

	svc := NewMentionService(&fakeMentionRelations{}, &fakeMentionSearch{available: true}, zap.NewNop())

	list, total, err := svc.SuggestFollowUsers(context.Background(), 7, "ali", 1, 20)
	require.NoError(t, err, "SuggestFollowUsers: %v", err)

	require.EqualValues(t, 0, total, "list = %v, total = %d", list, total)
	require.NotNil(t, list, "list = %v, total = %d", list, total)
	require.Empty(t, list, "list = %v, total = %d", list, total)
}

func TestSuggestFollowUsersPropagatesRelationError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("db down")
	svc := NewMentionService(&fakeMentionRelations{err: wantErr}, &fakeMentionSearch{}, zap.NewNop())

	{
		_, _, err := svc.SuggestFollowUsers(context.Background(), 7, "", 1, 20)
		require.ErrorIs(t, err, wantErr, "err = %v, want %v", err, wantErr)
	}
}

// mentionPage 构造测试用的规范化分页参数。
func mentionPage(page, pageSize int) domain.Pagination {
	return domain.NormalizeMentionPagination(page, pageSize)
}

func mentionIDs(users []MentionUser) []uint {
	ids := make([]uint, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.ID)
	}

	return ids
}
