package clienterror

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	videoerrors "f_ccg_video_back_ddd/internal/domain/video/domainerr"
)

func TestMessageDoesNotExposeInfrastructureOrWrappedDetails(t *testing.T) {
	for _, err := range []error{errors.New("SELECT password FROM users at 10.0.0.1"), fmt.Errorf("private-object-key: %w", videoerrors.ErrVideoNotFound)} {
		got := Message(err)
		if strings.Contains(got, "password") || strings.Contains(got, "private-object") || strings.Contains(got, "10.0.") {
			t.Fatalf("leaked details: %s", got)
		}
	}
	if got := Message(fmt.Errorf("database detail: %w", videoerrors.ErrVideoNotFound)); got != videoerrors.ErrVideoNotFound.Error() {
		t.Fatalf("lost business message: %s", got)
	}
}
