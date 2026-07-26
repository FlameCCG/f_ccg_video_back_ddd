package search

import (
	"context"
	"errors"
	"fmt"

	app "f_ccg_video_back_ddd/internal/application/search"
	domain "f_ccg_video_back_ddd/internal/domain/search"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/olivere/elastic/v7"
)

// UserIndexer 把用户搜索文档批量写入 search_users 索引。
//
// 文档字段名、`users:{id}` 文档 ID 与 refresh=wait_for 语义都属于外部契约：
// 关注/取关接口依赖「写完立即可被 @ 联想查到」。
type UserIndexer struct {
	client *elastic.Client
}

var _ app.UserDocumentIndexer = (*UserIndexer)(nil)

// NewUserIndexer 构造用户索引写入适配器（client 允许为 nil）。
func NewUserIndexer(client *elastic.Client) *UserIndexer {
	return &UserIndexer{client: client}
}

// Available 报告 ES 客户端是否就绪。
func (i *UserIndexer) Available() bool {
	return i != nil && i.client != nil
}

// BulkIndexUsers 批量覆盖写入用户文档并等待索引可见。
func (i *UserIndexer) BulkIndexUsers(ctx context.Context, docs []app.UserSearchDocument) error {
	if !i.Available() || len(docs) == 0 {
		return nil
	}

	bulk := i.client.Bulk().Index(constants.UserSearchIndex).Refresh("wait_for")

	for _, doc := range docs {
		if doc.ID == 0 {
			return errors.New("invalid user search document id")
		}

		bulk = bulk.Add(
			elastic.NewBulkIndexRequest().
				Id(domain.UserDocumentID(doc.ID)).
				Doc(buildUserSearchPayload(doc)),
		)
	}

	resp, err := bulk.Do(ctx)
	if err != nil {
		return err
	}

	if resp != nil && resp.Errors {
		for _, item := range resp.Items {
			for _, result := range item {
				if result == nil || result.Error == nil {
					continue
				}

				return fmt.Errorf("bulk sync user doc failed: %s", result.Error.Reason)
			}
		}

		return errors.New("bulk sync user doc failed")
	}

	return nil
}

// buildUserSearchPayload 生成 search_users 文档体（字段名逐字对应索引 mapping）。
func buildUserSearchPayload(doc app.UserSearchDocument) map[string]any {
	payload := map[string]any{
		"id":               doc.ID,
		"username":         doc.Username,
		"description":      doc.Description,
		"avatar":           doc.Avatar,
		"level":            doc.Level,
		"created_at":       doc.CreatedAt,
		"updated_at":       doc.UpdatedAt,
		"follower_count":   doc.FollowerCount,
		"follow_usernames": doc.FollowUsernames,
		"friend_usernames": doc.FriendUsernames,
	}

	if sugg := buildCompletionPayload([]string{doc.Username}); sugg != nil {
		payload["username_suggest"] = sugg
	}

	return payload
}

// buildCompletionPayload 生成 ES completion 字段的输入格式 {"input": [...]}；全空时返回 nil。
func buildCompletionPayload(values []string) map[string]any {
	inputs := domain.TrimNonEmpty(values)
	if len(inputs) == 0 {
		return nil
	}

	return map[string]any{"input": inputs}
}
