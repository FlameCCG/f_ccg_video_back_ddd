package contentops

import (
	"errors"
	"testing"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

func TestCalcRates(t *testing.T) {
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
			got := CalcRates(tc.values)
			if len(got) != len(tc.want) {
				t.Fatalf("unexpected length: got %d want %d", len(got), len(tc.want))
			}

			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("rates[%d]=%v, want %v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestResolveBannerPartitionID(t *testing.T) {
	t.Run("非首页类型携带分区报错", func(t *testing.T) {
		_, _, err := ResolveBannerPartitionID(enum.BannerTypeTop, 3)
		if !errors.Is(err, ErrBannerPartitionTypeUnsupported) {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("非首页类型分区归零", func(t *testing.T) {
		resolved, needExists, err := ResolveBannerPartitionID(enum.BannerTypeUser, 0)
		if err != nil || resolved != 0 || needExists {
			t.Fatalf("unexpected result: resolved=%d needExists=%v err=%v", resolved, needExists, err)
		}
	})

	t.Run("首页本身无需校验分区", func(t *testing.T) {
		resolved, needExists, err := ResolveBannerPartitionID(enum.BannerTypeHome, 0)
		if err != nil || resolved != 0 || needExists {
			t.Fatalf("unexpected result: resolved=%d needExists=%v err=%v", resolved, needExists, err)
		}
	})

	t.Run("首页分区需要校验存在性", func(t *testing.T) {
		resolved, needExists, err := ResolveBannerPartitionID(enum.BannerTypeHome, 7)
		if err != nil || resolved != 7 || !needExists {
			t.Fatalf("unexpected result: resolved=%d needExists=%v err=%v", resolved, needExists, err)
		}
	})
}

func TestBannerAdminQueryNormalize(t *testing.T) {
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

		if query.Page != tc.wantPage || query.PageSize != tc.wantPageSize {
			t.Fatalf("normalize(%+v) = page %d size %d, want page %d size %d",
				tc.in, query.Page, query.PageSize, tc.wantPage, tc.wantPageSize)
		}
	}
}

func TestBannerChangesIsEmpty(t *testing.T) {
	if !(BannerChanges{}).IsEmpty() {
		t.Fatal("zero changes should be empty")
	}

	cover := "c"
	if (BannerChanges{Cover: &cover}).IsEmpty() {
		t.Fatal("changes with cover should not be empty")
	}

	partitionID := uint(0)
	if (BannerChanges{PartitionID: &partitionID}).IsEmpty() {
		t.Fatal("changes with zero partition pointer should not be empty")
	}
}
