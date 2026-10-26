package dynamic

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmptyDeepPagePreservesFeedTotal(t *testing.T) {
	t.Parallel()

	total, rows, err := resolveFeedTotal(context.Background(), nil, 100, 20, func(context.Context) (int64, error) { return 42, nil })
	require.NoError(t, err, "total=%d rows=%v err=%v", total, rows, err)
	require.EqualValues(t, 42, total, "total=%d rows=%v err=%v", total, rows, err)
	require.NotNil(t, rows, "total=%d rows=%v err=%v", total, rows, err)
	require.Empty(t, rows, "total=%d rows=%v err=%v", total, rows, err)

	failed := errors.New("count failure")
	{
		_, _, err := resolveFeedTotal(context.Background(), nil, 100, 20, func(context.Context) (int64, error) { return 0, failed })
		require.ErrorIs(t, err, failed, "count error lost: %v", err)
	}
}
