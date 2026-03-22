package identity

import (
	"testing"
	"time"
)

func TestPlanAutoUnban(t *testing.T) {
	endAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)

	plan := (AutoUnbanPolicy{}).PlanAutoUnban([]ExpiredBan{
		{RecordID: 1, UserID: 0, EndAt: &endAt},
		{RecordID: 0, UserID: 8, EndAt: &endAt},
		{RecordID: 7, UserID: 9, EndAt: &endAt},
	})

	if plan.Empty() {
		t.Fatal("plan should not be empty")
	}

	if len(plan.UserIDs) != 1 || plan.UserIDs[0] != 9 {
		t.Fatalf("expected only user 9, got %v", plan.UserIDs)
	}

	if len(plan.RecordIDs) != 1 || plan.RecordIDs[0] != 7 {
		t.Fatalf("expected only record 7, got %v", plan.RecordIDs)
	}
}

func TestPlanAutoUnbanEmptyInputs(t *testing.T) {
	if !(AutoUnbanPolicy{}).PlanAutoUnban(nil).Empty() {
		t.Fatal("nil input should produce an empty plan")
	}

	// 全部为非法用户或记录 ID 时同样视为无事可做。
	if !(AutoUnbanPolicy{}).PlanAutoUnban([]ExpiredBan{{RecordID: 1, UserID: 0}}).Empty() {
		t.Fatal("all-invalid input should produce an empty plan")
	}
}
