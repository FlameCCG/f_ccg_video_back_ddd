package identity_test

import (
	"errors"
	"testing"

	identity "f_ccg_video_back_ddd/internal/domain/identity"
)

func TestCoinChangeRoundingAndBalanceUseTheSameAmount(t *testing.T) {
	for _, tc := range []struct{ raw, rounded float64 }{
		{0, 0}, {.04, 0}, {-.04, 0}, {.05, .1}, {-.05, -.1}, {.24, .2}, {-.25, -.3},
	} {
		change := identity.NewCoinChange(tc.raw)
		if change.Amount() != tc.rounded || change.IsZero() != (tc.rounded == 0) {
			t.Fatalf("raw=%v amount=%v zero=%v", tc.raw, change.Amount(), change.IsZero())
		}
		account := identity.RestoreCoinAccount(1, 1)
		if err := account.ApplyChange(change); err != nil || account.Balance() != identity.NewCoinChange(1+tc.rounded).Amount() {
			t.Fatalf("raw=%v balance=%v err=%v", tc.raw, account.Balance(), err)
		}
	}
	// 重建保留历史精度；取整为零的动作不偷偷重写历史余额。
	account := identity.RestoreCoinAccount(1, .123)
	if err := account.ApplyChange(identity.NewCoinChange(.04)); err != nil || account.Balance() != .123 {
		t.Fatal(account.Balance(), err)
	}
	before := account.Balance()
	if err := account.ApplyChange(identity.NewCoinChange(-1)); !errors.Is(err, identity.ErrCoinInsufficient) || account.Balance() != before {
		t.Fatal("rejected spending changed historical balance", account.Balance(), err)
	}
}
