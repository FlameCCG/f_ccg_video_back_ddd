package search

import (
	"context"
	"errors"
	"sort"

	searchdomain "f_ccg_video_back_ddd/internal/domain/search"

	"go.uber.org/zap"
)

// ErrInvalidMentionUserID 表示调用方未提供有效的当前用户 ID.
var ErrInvalidMentionUserID = errors.New("invalid user id")

// MentionService 评论 @ 提及联想用例服务。
//
// 候选集合来自库内关注关系（保证只能 @ 到自己关注的人），
// 关键词过滤优先用 ES 的用户名/拼音匹配，ES 不可用时退化为本地字面匹配。
type MentionService struct {
	relations MentionRelationQuery
	index     MentionSearch
	logger    *zap.Logger
	keywords  searchdomain.KeywordPolicy
	pages     searchdomain.PaginationPolicy
}

// NewMentionService 构造 @ 提及联想服务。
func NewMentionService(relations MentionRelationQuery, index MentionSearch, logger *zap.Logger) *MentionService {
	return &MentionService{
		relations: relations,
		index:     index,
		logger:    logger,
		keywords:  searchdomain.KeywordPolicy{},
		pages:     searchdomain.PaginationPolicy{},
	}
}

// SuggestFollowUsers 返回当前用户关注列表中匹配关键词的用户（好友优先），以及匹配总量。
func (s *MentionService) SuggestFollowUsers(
	ctx context.Context,
	userID uint,
	keyword string,
	page, pageSize int,
) ([]MentionUser, int64, error) {
	if userID == 0 {
		return nil, 0, ErrInvalidMentionUserID
	}

	ctx = (callContext{}).ensure(ctx)
	pagination := s.pages.NormalizeMentionPagination(page, pageSize)
	keyword = s.keywords.NormalizeKeyword(keyword)

	users, err := s.relations.FollowingWithFriendFlag(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	if len(users) == 0 {
		return []MentionUser{}, 0, nil
	}

	if keyword == "" {
		s.sortMentionUsers(users)

		return s.paginateMentionUsers(users, pagination), int64(len(users)), nil
	}

	matchedUsers, err := s.filterByKeyword(ctx, users, keyword)
	if err != nil {
		return nil, 0, err
	}

	s.sortMentionUsers(matchedUsers)

	return s.paginateMentionUsers(matchedUsers, pagination), int64(len(matchedUsers)), nil
}

// filterByKeyword 合并 ES 命中与本地字面命中；ES 出错只记日志并退化为本地匹配。
func (s *MentionService) filterByKeyword(
	ctx context.Context,
	users []MentionUser,
	keyword string,
) ([]MentionUser, error) {
	merged := make(map[uint]MentionUser, len(users))

	if s.index.Available() {
		userMap := make(map[uint]MentionUser, len(users))

		ids := make([]uint, 0, len(users))
		for _, user := range users {
			ids = append(ids, user.ID)
			userMap[user.ID] = user
		}

		matchedIDs, err := s.index.MatchUserIDs(ctx, ids, keyword)
		if err != nil {
			s.logger.Warn("mention suggest fallback to local match", zap.Error(err))
		} else {
			for _, id := range matchedIDs {
				if user, ok := userMap[id]; ok {
					merged[id] = user
				}
			}
		}
	}

	for _, user := range users {
		if !s.keywords.MatchesUsername(user.Username, keyword) {
			continue
		}

		merged[user.ID] = user
	}

	if len(merged) == 0 {
		return []MentionUser{}, nil
	}

	result := make([]MentionUser, 0, len(merged))
	for _, user := range merged {
		result = append(result, user)
	}

	s.sortMentionUsers(result)

	return result, nil
}

// sortMentionUsers 排序规则：互关好友优先 → 粉丝数降序 → ID 升序。
func (s *MentionService) sortMentionUsers(users []MentionUser) {
	order := searchdomain.MentionOrder{}
	sort.SliceStable(users, func(i, j int) bool {
		return order.Less(searchdomain.MentionCandidate{ID: users[i].ID, IsFriend: users[i].IsFriend, FollowerCount: users[i].FollowerCount}, searchdomain.MentionCandidate{ID: users[j].ID, IsFriend: users[j].IsFriend, FollowerCount: users[j].FollowerCount})
	})
}

// paginateMentionUsers 内存分页；越界返回空列表。
func (s *MentionService) paginateMentionUsers(users []MentionUser, pagination searchdomain.Pagination) []MentionUser {
	start, end := s.pages.SliceWindow(len(users), pagination)
	if start == end {
		return []MentionUser{}
	}

	return users[start:end]
}
