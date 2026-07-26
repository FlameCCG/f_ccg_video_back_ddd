package search

import (
	"context"
	"slices"
	"time"
)

// userDocumentSyncTimeout 用户搜索文档同步超时（含读库与 bulk 写入）.
const userDocumentSyncTimeout = 5 * time.Second

// UserIndexService 用户搜索文档同步用例服务（原 service/search_service/user_sync.go）。
//
// 关注/取关后立即刷新 search_users 文档，保证 @ 提及联想与用户搜索的字段及时可见。
type UserIndexService struct {
	query   UserSearchDocumentQuery
	indexer UserDocumentIndexer
}

// NewUserIndexService 构造用户索引同步服务。
func NewUserIndexService(query UserSearchDocumentQuery, indexer UserDocumentIndexer) *UserIndexService {
	return &UserIndexService{query: query, indexer: indexer}
}

// SyncUserDocuments 刷新给定用户在 search_users 索引中的文档；ES 不可用时直接跳过。
func (s *UserIndexService) SyncUserDocuments(ctx context.Context, userIDs ...uint) error {
	if !s.indexer.Available() {
		return nil
	}

	ids := uniqueUserIDs(userIDs)
	if len(ids) == 0 {
		return nil
	}

	syncCtx, cancel := context.WithTimeout(ensureContext(ctx), userDocumentSyncTimeout)
	defer cancel()

	docs, err := s.query.LoadDocuments(syncCtx, ids)
	if err != nil {
		return err
	}

	if len(docs) == 0 {
		return nil
	}

	return s.indexer.BulkIndexUsers(syncCtx, docs)
}

// uniqueUserIDs 去掉 0 值、去重并升序排序（保证 bulk 请求顺序稳定）。
func uniqueUserIDs(userIDs []uint) []uint {
	if len(userIDs) == 0 {
		return nil
	}

	seen := make(map[uint]struct{}, len(userIDs))

	ids := make([]uint, 0, len(userIDs))

	for _, userID := range userIDs {
		if userID == 0 {
			continue
		}

		if _, ok := seen[userID]; ok {
			continue
		}

		seen[userID] = struct{}{}
		ids = append(ids, userID)
	}

	slices.Sort(ids)

	return ids
}
