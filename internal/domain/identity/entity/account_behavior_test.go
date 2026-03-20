package entity_test

import (
	"errors"
	"testing"

	identityerrors "f_ccg_video_back_ddd/internal/domain/identity/domainerr"
	"f_ccg_video_back_ddd/internal/domain/identity/entity"
)

func TestCoinChangesProtectBalanceAndPermitRewardDebt(t *testing.T) {
	user := &entity.User{ID: 1, CoinCount: 0}
	if err := user.ApplyCoinChange(-0.1); !errors.Is(err, identityerrors.ErrCoinInsufficient) {
		t.Fatalf("spending error = %v", err)
	}
	if user.CoinCount != 0 {
		t.Fatalf("rejected spending changed balance: %v", user.CoinCount)
	}
	if err := user.RevokeFavoriteReward(); err != nil || user.CoinCount != -0.1 {
		t.Fatalf("reward reversal: balance=%v err=%v", user.CoinCount, err)
	}
	if err := user.ApplyCoinChange(-0.1); !errors.Is(err, identityerrors.ErrCoinInsufficient) || user.CoinCount != -0.1 {
		t.Fatalf("debt must block further spending: balance=%v err=%v", user.CoinCount, err)
	}
	if err := user.RevokeFavoriteReward(); err != nil || user.CoinCount != -0.2 {
		t.Fatalf("another reward reversal: balance=%v err=%v", user.CoinCount, err)
	}
	if err := user.ApplyCoinChange(0.1); err != nil || user.CoinCount != -0.1 {
		t.Fatalf("partial debt repayment: balance=%v err=%v", user.CoinCount, err)
	}
	if err := user.ApplyCoinChange(0.23); err != nil || user.CoinCount != 0.1 {
		t.Fatalf("repayment must retain decimal rounding: balance=%v err=%v", user.CoinCount, err)
	}
}

func TestExperienceChangesKeepLevelAndDailyLimitTogether(t *testing.T) {
	user := &entity.User{ID: 1, Exp: 995, Level: 1}
	if applied, ok := user.ApplyExperience(15, 55); !ok || applied != 5 || user.Exp != 1000 || user.Level != 2 {
		t.Fatalf("capped gain must advance level: applied=%d ok=%v user=%+v", applied, ok, user)
	}
	before := *user
	if applied, ok := user.ApplyExperience(10, 60); ok || applied != 0 || *user != before {
		t.Fatalf("daily cap must leave state unchanged: applied=%d ok=%v user=%+v", applied, ok, user)
	}
	if applied, ok := user.ApplyExperience(-2000, 60); !ok || applied != -2000 || user.Exp != 0 || user.Level != 1 {
		t.Fatalf("negative adjustment preserves ledger delta and floors experience: applied=%d ok=%v user=%+v", applied, ok, user)
	}
}
