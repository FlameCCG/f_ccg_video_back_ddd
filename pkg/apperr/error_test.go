package apperr

import (
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestWrapPreservesBothBranchesAndOriginalTypes(t *testing.T) {
	t.Parallel()

	business := errors.New("safe rejection")
	cause := &os.PathError{Op: "open", Path: "private", Err: os.ErrPermission}

	wrapped := fmt.Errorf("outer: %w", Wrap("save", business, cause, Dependency))
	for _, target := range []error{business, cause, os.ErrPermission} {
		if !errors.Is(wrapped, target) {
			t.Fatalf("lost error identity %v", target)
		}
	}

	var pathErr *os.PathError
	if !errors.As(wrapped, &pathErr) || pathErr != cause {
		t.Fatal("lost original cause type")
	}

	outer := Wrap("request", nil, wrapped, Degraded)
	if got := Semantic(outer); got == nil || got.Kind != Degraded {
		t.Fatalf("outer semantics lost: %#v", got)
	}
}
