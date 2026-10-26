package contentops

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalcRates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		values []int64
		want   []float64
	}{
		{name: "empty", values: nil, want: []float64{}},
		{name: "first point is zero", values: []int64{5}, want: []float64{0}},
		{name: "zero to zero", values: []int64{0, 0}, want: []float64{0, 0}},
		{name: "zero to positive is 100", values: []int64{0, 3}, want: []float64{0, 100}},
		{name: "growth", values: []int64{2, 3}, want: []float64{0, 50}},
		{name: "decline", values: []int64{4, 1}, want: []float64{0, -75}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := CalcRates(tc.values)
			require.Len(t, tc.want, len(got), "unexpected length: got %d want %d", len(got), len(tc.want))

			for i := range tc.want {
				require.InDelta(t, tc.want[i], got[i], 1e-9, "rates[%d]=%v, want %v", i, got[i], tc.want[i])
			}
		})
	}
}

func TestResolveBannerPartitionID(t *testing.T) {
	t.Parallel()

	t.Run("非首页类型携带分区报错", func(t *testing.T) {
		t.Parallel()

		_, _, err := ResolveBannerPartitionID(enum.BannerTypeTop, 3)
		require.ErrorIs(t, err, ErrBannerPartitionTypeUnsupported, "unexpected err: %v", err)
	})

	t.Run("非首页类型分区归零", func(t *testing.T) {
		t.Parallel()

		resolved, needExists, err := ResolveBannerPartitionID(enum.BannerTypeUser, 0)
		require.NoError(t, err, "unexpected result: resolved=%d needExists=%v err=%v", resolved, needExists, err)
		require.EqualValues(t, 0, resolved, "unexpected result: resolved=%d needExists=%v err=%v", resolved, needExists, err)
		require.False(t, needExists, "unexpected result: resolved=%d needExists=%v err=%v", resolved, needExists, err)
	})

	t.Run("首页本身无需校验分区", func(t *testing.T) {
		t.Parallel()

		resolved, needExists, err := ResolveBannerPartitionID(enum.BannerTypeHome, 0)
		require.NoError(t, err, "unexpected result: resolved=%d needExists=%v err=%v", resolved, needExists, err)
		require.EqualValues(t, 0, resolved, "unexpected result: resolved=%d needExists=%v err=%v", resolved, needExists, err)
		require.False(t, needExists, "unexpected result: resolved=%d needExists=%v err=%v", resolved, needExists, err)
	})

	t.Run("首页分区需要校验存在性", func(t *testing.T) {
		t.Parallel()

		resolved, needExists, err := ResolveBannerPartitionID(enum.BannerTypeHome, 7)
		require.NoError(t, err, "unexpected result: resolved=%d needExists=%v err=%v", resolved, needExists, err)
		require.EqualValues(t, 7, resolved, "unexpected result: resolved=%d needExists=%v err=%v", resolved, needExists, err)
		require.True(t, needExists, "unexpected result: resolved=%d needExists=%v err=%v", resolved, needExists, err)
	})
}

func TestBannerAdminQueryNormalize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in           BannerAdminQuery
		wantPage     int
		wantPageSize int
	}{
		{in: BannerAdminQuery{}, wantPage: 1, wantPageSize: 20},
		{in: BannerAdminQuery{Page: -3, PageSize: 101}, wantPage: 1, wantPageSize: 20},
		{in: BannerAdminQuery{Page: 2, PageSize: 100}, wantPage: 2, wantPageSize: 100},
	}

	for _, tc := range cases {
		query := tc.in
		query.Normalize()

		require.Equal(t, tc.wantPage, query.Page, "normalize(%+v) = page %d size %d, want page %d size %d", tc.in, query.Page, query.PageSize, tc.wantPage, tc.wantPageSize)
		require.Equal(t, tc.wantPageSize, query.PageSize, "normalize(%+v) = page %d size %d, want page %d size %d", tc.in, query.Page, query.PageSize, tc.wantPage, tc.wantPageSize)
	}
}

func TestBannerChangesIsEmpty(t *testing.T) {
	t.Parallel()

	require.True(t, (BannerChanges{}).IsEmpty(), "zero changes should be empty")

	cover := "c"
	require.False(t, (BannerChanges{Cover: &cover}).IsEmpty(), "changes with cover should not be empty")

	partitionID := uint(0)
	require.False(t, (BannerChanges{PartitionID: &partitionID}).IsEmpty(), "changes with zero partition pointer should not be empty")
}
