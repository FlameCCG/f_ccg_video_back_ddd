package search

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// 对应原 service/search_service/user_sync.go（原文件无单测，此处按分支补齐）。

func TestUniqueUserIDsDropsZeroAndSorts(t *testing.T) {
	t.Parallel()

	if got := uniqueUserIDs([]uint{9, 0, 3, 9, 1, 0}); !reflect.DeepEqual(got, []uint{1, 3, 9}) {
		t.Fatalf("uniqueUserIDs = %v, want [1 3 9]", got)
	}

	if got := uniqueUserIDs(nil); got != nil {
		t.Fatalf("nil input should yield nil, got %v", got)
	}

	if got := uniqueUserIDs([]uint{0, 0}); got != nil && len(got) != 0 {
		t.Fatalf("all-zero input = %v", got)
	}
}

// TestSyncUserDocumentsSkipsWhenESUnavailable ES 不可用时直接返回 nil，不查库。
func TestSyncUserDocumentsSkipsWhenESUnavailable(t *testing.T) {
	t.Parallel()

	query := &fakeUserDocumentQuery{}
	svc := NewUserIndexService(query, &fakeUserDocumentIndexer{available: false})

	if err := svc.SyncUserDocuments(context.Background(), 1, 2); err != nil {
		t.Fatalf("SyncUserDocuments: %v", err)
	}

	if query.callCount != 0 {
		t.Fatalf("query must be skipped when ES is unavailable (calls=%d)", query.callCount)
	}
}

// TestSyncUserDocumentsSkipsEmptyIDSet 去重后没有有效 ID 时不查库、不写 ES。
func TestSyncUserDocumentsSkipsEmptyIDSet(t *testing.T) {
	t.Parallel()

	query := &fakeUserDocumentQuery{}
	indexer := &fakeUserDocumentIndexer{available: true}
	svc := NewUserIndexService(query, indexer)

	if err := svc.SyncUserDocuments(context.Background()); err != nil {
		t.Fatalf("SyncUserDocuments: %v", err)
	}

	if err := svc.SyncUserDocuments(context.Background(), 0, 0); err != nil {
		t.Fatalf("SyncUserDocuments: %v", err)
	}

	if query.callCount != 0 || len(indexer.batches) != 0 {
		t.Fatalf("calls=%d batches=%d", query.callCount, len(indexer.batches))
	}
}

// TestSyncUserDocumentsBulkIndexesDeduplicatedIDs 去重升序后的 ID 传给查询，
// 装载到的文档整批写入 ES。
func TestSyncUserDocumentsBulkIndexesDeduplicatedIDs(t *testing.T) {
	t.Parallel()

	docs := []UserSearchDocument{
		{ID: 1, Username: "alice", FollowerCount: 2},
		{ID: 5, Username: "bob"},
	}
	query := &fakeUserDocumentQuery{docs: docs}
	indexer := &fakeUserDocumentIndexer{available: true}
	svc := NewUserIndexService(query, indexer)

	if err := svc.SyncUserDocuments(context.Background(), 5, 1, 5); err != nil {
		t.Fatalf("SyncUserDocuments: %v", err)
	}

	if !reflect.DeepEqual(query.lastIDs, []uint{1, 5}) {
		t.Fatalf("queried ids = %v, want [1 5]", query.lastIDs)
	}

	if len(indexer.batches) != 1 || !reflect.DeepEqual(indexer.batches[0], docs) {
		t.Fatalf("batches = %+v", indexer.batches)
	}
}

// TestSyncUserDocumentsSkipsBulkWhenNoDocumentsLoaded 库里查不到任何用户时不发 bulk。
func TestSyncUserDocumentsSkipsBulkWhenNoDocumentsLoaded(t *testing.T) {
	t.Parallel()

	indexer := &fakeUserDocumentIndexer{available: true}
	svc := NewUserIndexService(&fakeUserDocumentQuery{}, indexer)

	if err := svc.SyncUserDocuments(context.Background(), 7); err != nil {
		t.Fatalf("SyncUserDocuments: %v", err)
	}

	if len(indexer.batches) != 0 {
		t.Fatalf("batches = %+v, want none", indexer.batches)
	}
}

func TestSyncUserDocumentsPropagatesErrors(t *testing.T) {
	t.Parallel()

	queryErr := errors.New("db down")
	svc := NewUserIndexService(
		&fakeUserDocumentQuery{err: queryErr},
		&fakeUserDocumentIndexer{available: true},
	)

	if err := svc.SyncUserDocuments(context.Background(), 1); !errors.Is(err, queryErr) {
		t.Fatalf("err = %v, want %v", err, queryErr)
	}

	bulkErr := errors.New("bulk failed")
	svc = NewUserIndexService(
		&fakeUserDocumentQuery{docs: []UserSearchDocument{{ID: 1}}},
		&fakeUserDocumentIndexer{available: true, err: bulkErr},
	)

	if err := svc.SyncUserDocuments(context.Background(), 1); !errors.Is(err, bulkErr) {
		t.Fatalf("err = %v, want %v", err, bulkErr)
	}
}
