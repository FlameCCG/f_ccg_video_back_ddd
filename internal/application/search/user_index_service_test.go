package search

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// 对应原 service/search_service/user_sync.go（原文件无单测，此处按分支补齐）。

func TestUniqueUserIDsDropsZeroAndSorts(t *testing.T) {
	t.Parallel()

	{
		got := uniqueUserIDs([]uint{9, 0, 3, 9, 1, 0})
		require.True(t, reflect.DeepEqual(got, []uint{1, 3, 9}), "uniqueUserIDs = %v, want [1 3 9]", got)
	}

	{
		got := uniqueUserIDs(nil)
		require.Nil(t, got, "nil input should yield nil, got %v", got)
	}

	{
		got := uniqueUserIDs([]uint{0, 0})
		require.Empty(t, got, "all-zero input = %v", got)
	}
}

// TestSyncUserDocumentsSkipsWhenESUnavailable ES 不可用时直接返回 nil，不查库。
func TestSyncUserDocumentsSkipsWhenESUnavailable(t *testing.T) {
	t.Parallel()

	query := &fakeUserDocumentQuery{}
	svc := NewUserIndexService(query, &fakeUserDocumentIndexer{available: false})

	{
		err := svc.SyncUserDocuments(context.Background(), 1, 2)
		require.NoError(t, err, "SyncUserDocuments: %v", err)
	}

	require.Equal(t, 0, query.callCount, "query must be skipped when ES is unavailable (calls=%d)", query.callCount)
}

// TestSyncUserDocumentsSkipsEmptyIDSet 去重后没有有效 ID 时不查库、不写 ES。
func TestSyncUserDocumentsSkipsEmptyIDSet(t *testing.T) {
	t.Parallel()

	query := &fakeUserDocumentQuery{}
	indexer := &fakeUserDocumentIndexer{available: true}
	svc := NewUserIndexService(query, indexer)

	{
		err := svc.SyncUserDocuments(context.Background())
		require.NoError(t, err, "SyncUserDocuments: %v", err)
	}

	{
		err := svc.SyncUserDocuments(context.Background(), 0, 0)
		require.NoError(t, err, "SyncUserDocuments: %v", err)
	}

	require.False(t, query.callCount != 0 || len(indexer.batches) != 0, "calls=%d batches=%d", query.callCount, len(indexer.batches))
}

// TestSyncUserDocumentsBulkIndexesDeduplicatedIDs 去重升序后的 ID 传给查询，
// 装载到的文档整批写入 ES。
func TestSyncUserDocumentsBulkIndexesDeduplicatedIDs(t *testing.T) {
	t.Parallel()

	docs := []UserSearchDocument{
		{ID: 1, Username: testAlice, FollowerCount: 2},
		{ID: 5, Username: "bob"},
	}
	query := &fakeUserDocumentQuery{docs: docs}
	indexer := &fakeUserDocumentIndexer{available: true}
	svc := NewUserIndexService(query, indexer)

	{
		err := svc.SyncUserDocuments(context.Background(), 5, 1, 5)
		require.NoError(t, err, "SyncUserDocuments: %v", err)
	}

	require.True(t, reflect.DeepEqual(query.lastIDs, []uint{1, 5}), "queried ids = %v, want [1 5]", query.lastIDs)

	require.Len(t, indexer.batches, 1, "batches = %+v", indexer.batches)
	require.True(t, reflect.DeepEqual(indexer.batches[0], docs), "batches = %+v", indexer.batches)
}

// TestSyncUserDocumentsSkipsBulkWhenNoDocumentsLoaded 库里查不到任何用户时不发 bulk。
func TestSyncUserDocumentsSkipsBulkWhenNoDocumentsLoaded(t *testing.T) {
	t.Parallel()

	indexer := &fakeUserDocumentIndexer{available: true}
	svc := NewUserIndexService(&fakeUserDocumentQuery{}, indexer)

	{
		err := svc.SyncUserDocuments(context.Background(), 7)
		require.NoError(t, err, "SyncUserDocuments: %v", err)
	}

	require.Empty(t, indexer.batches, "batches = %+v, want none", indexer.batches)
}

func TestSyncUserDocumentsPropagatesErrors(t *testing.T) {
	t.Parallel()

	queryErr := errors.New("db down")
	svc := NewUserIndexService(
		&fakeUserDocumentQuery{err: queryErr},
		&fakeUserDocumentIndexer{available: true},
	)

	{
		err := svc.SyncUserDocuments(context.Background(), 1)
		require.ErrorIs(t, err, queryErr, "err = %v, want %v", err, queryErr)
	}

	bulkErr := errors.New("bulk failed")
	svc = NewUserIndexService(
		&fakeUserDocumentQuery{docs: []UserSearchDocument{{ID: 1}}},
		&fakeUserDocumentIndexer{available: true, err: bulkErr},
	)

	{
		err := svc.SyncUserDocuments(context.Background(), 1)
		require.ErrorIs(t, err, bulkErr, "err = %v, want %v", err, bulkErr)
	}
}
