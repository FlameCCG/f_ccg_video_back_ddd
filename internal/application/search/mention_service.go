package search

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/logctx"
	"sort"

	domain "f_ccg_video_back_ddd/internal/domain/search"

	"go.uber.org/zap"
)

// ErrInvalidMentionUserID 表示调用方未提供有效的当前用户 ID.
var ErrInvalidMentionUserID = errors.New("invalid user id")

// MentionService 评论 @ 提及联想用例服务（原 service/search_service/mention.go）。
//
// 候选集合来自库内关注关系（保证只能 @ 到自己关注的人），
// 关键词过滤优先用 ES 的用户名/拼音匹配，ES 不可用时退化为本地字面匹配。
type MentionService struct {
	relations MentionRelationQuery
	index     MentionSearch
	logger    *zap.Logger
}

// NewMentionService 构造 @ 提及联想服务。
func NewMentionService(relations MentionRelationQuery, index MentionSearch, logger *zap.Logger) *MentionService {
	return &MentionService{relations: relations, index: index, logger: logger}
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

	ctx = ensureContext(ctx)
	pagination := domain.NormalizeMentionPagination(page, pageSize)
	keyword = domain.NormalizeKeyword(keyword)

	users, err := s.relations.FollowingWithFriendFlag(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	if len(users) == 0 {
		return []MentionUser{}, 0, nil
	}

	if keyword == "" {
		sortMentionUsers(users)

		return paginateMentionUsers(users, pagination), int64(len(users)), nil
	}

	matchedUsers, err := s.filterByKeyword(ctx, users, keyword)
	if err != nil {
		return nil, 0, err
	}

	sortMentionUsers(matchedUsers)

	return paginateMentionUsers(matchedUsers, pagination), int64(len(matchedUsers)), nil
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
			logctx.With(ctx, s.logger).Warn("mention suggest fallback to local match", zap.String("event", "search.mention_service.filter_by_keyword.degraded"), zap.String("operation", "search.mention_service.filter_by_keyword"), zap.Error(err))
		} else {
			for _, id := range matchedIDs {
				if user, ok := userMap[id]; ok {
					merged[id] = user
				}
			}
		}
	}

	for _, user := range users {
		if !domain.MatchesUsername(user.Username, keyword) {
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

	sortMentionUsers(result)

	return result, nil
}

// sortMentionUsers 排序规则：互关好友优先 → 粉丝数降序 → ID 升序。
func sortMentionUsers(users []MentionUser) {
	sort.SliceStable(users, func(i, j int) bool {
		if users[i].IsFriend != users[j].IsFriend {
			return users[i].IsFriend && !users[j].IsFriend
		}

		if users[i].FollowerCount != users[j].FollowerCount {
			return users[i].FollowerCount > users[j].FollowerCount
		}

		return users[i].ID < users[j].ID
	})
}

// paginateMentionUsers 内存分页；越界返回空列表。
func paginateMentionUsers(users []MentionUser, pagination domain.Pagination) []MentionUser {
	start, end := domain.SliceWindow(len(users), pagination)
	if start == end {
		return []MentionUser{}
	}

	return users[start:end]
}
