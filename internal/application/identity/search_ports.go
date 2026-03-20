package identity

import (
	"context"
)

// 用户搜索排序字段/方向（取值必须与 search 上下文一致）。
const (
	// UserSortRelevance 按相关性（ES _score）排序。
	UserSortRelevance = 0
	// UserSortFollowers 按粉丝数排序。
	UserSortFollowers = 1
	// UserSortLevel 按等级排序。
	UserSortLevel = 2
	// UserSortCreatedAt 按注册时间排序。
	UserSortCreatedAt = 3

	// SortOrderDesc 降序。
	SortOrderDesc = 0
	// SortOrderAsc 升序。
	SortOrderAsc = 1
)

// UserSearchParams 用户搜索参数。
type UserSearchParams struct {
	Keyword   string
	Page      int
	PageSize  int
	UserSort  int
	UserOrder int
}

// UserSearchQuery 用户搜索端口。
//
// 跨上下文依赖：应绑定到 search 上下文。
// 返回值原样交给 res.OkWithData，实现须返回 search 上下文自己的 *UserSearchResult。
type UserSearchQuery interface {
	SearchUsers(ctx context.Context, params UserSearchParams) (any, error)
}

// MentionSuggestQuery 评论 @ 联想端口。
//
// 跨上下文依赖：应绑定到 search 上下文。
// 返回的 items 原样交给 res.OkWithList，实现须返回 search 上下文自己的 []MentionUser。
type MentionSuggestQuery interface {
	SuggestFollowUsers(ctx context.Context, userID uint, keyword string, page, pageSize int) (items any, total int64, err error)
}
