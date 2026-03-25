package chat

import (
	"errors"
	"testing"
)

func TestDailySendPolicyIncludesFollowExemption(t *testing.T) {
	for _, tc := range []struct {
		facts DailySendFacts
		want  error
	}{
		{DailySendFacts{ReceiverFollowsSender: true, SentToday: true}, nil},
		{DailySendFacts{}, nil},
		{DailySendFacts{SentToday: true}, ErrDailySendLimit},
		{DailySendFacts{SentToday: true, RepliedToday: true}, nil},
	} {
		if err := (SendLimit{}).EvaluateDailySendLimit(tc.facts); !errors.Is(err, tc.want) {
			t.Fatalf("facts=%+v: %v, want %v", tc.facts, err, tc.want)
		}
	}
}
