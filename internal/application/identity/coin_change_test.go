package identity_test

import (
	"errors"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/identity"
	identity "f_ccg_video_back_ddd/internal/domain/identity"
)

func TestRoundedZeroCoinChangesDoNotAccessTransactionsOrRepositories(t *testing.T) {
	// nil 依赖会在被访问时失败，验证零变动在任何数据库访问前结束。
	coins := app.NewCoinService(nil, nil, nil)
	records := app.NewCoinRecordService(nil, nil)
	for _, amount := range []float64{0, .04, -.04} {
		result, err := coins.ApplyChange(t.Context(), 1, amount, "", nil)
		if err != nil || result.Applied || result.Balance != 0 || result.UserID != 1 {
			t.Fatalf("amount=%v result=%+v err=%v", amount, result, err)
		}
		if err := records.Record(t.Context(), 1, amount, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := coins.ApplyChange(t.Context(), 0, .04, "", nil); !errors.Is(err, identity.ErrUserNotLogin) {
		t.Fatalf("zero amount must not bypass login validation: %v", err)
	}
	if err := records.Record(t.Context(), 0, .04, ""); !errors.Is(err, identity.ErrUserNotLogin) {
		t.Fatal(err)
	}
}
