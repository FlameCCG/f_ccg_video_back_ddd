package dynamic

import (
	"context"
	"errors"
	"testing"
)

func TestEmptyDeepPagePreservesFeedTotal(t *testing.T) {
	total, rows, err := resolveFeedTotal(context.Background(), nil, 100, 20, func(context.Context) (int64, error) { return 42, nil })
	if err != nil || total != 42 || rows == nil || len(rows) != 0 {
		t.Fatalf("total=%d rows=%v err=%v", total, rows, err)
	}
	failed := errors.New("count failure")
	if _, _, err := resolveFeedTotal(context.Background(), nil, 100, 20, func(context.Context) (int64, error) { return 0, failed }); !errors.Is(err, failed) {
		t.Fatalf("count error lost: %v", err)
	}
}
