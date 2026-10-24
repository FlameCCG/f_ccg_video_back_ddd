package clienterror

import (
	"errors"
	video "f_ccg_video_back_ddd/internal/domain/video"
	"fmt"
	"strings"
	"testing"
)

func TestMessageDoesNotExposeInfrastructureOrWrappedDetails(t *testing.T) {
	for _, err := range []error{errors.New("SELECT password FROM users at 10.0.0.1"), fmt.Errorf("private-object-key: %w", video.ErrVideoNotFound)} {
		got := Message(err)
		if strings.Contains(got, "password") || strings.Contains(got, "private-object") || strings.Contains(got, "10.0.") {
			t.Fatalf("leaked details: %s", got)
		}
	}
	if got := Message(fmt.Errorf("database detail: %w", video.ErrVideoNotFound)); got != video.ErrVideoNotFound.Error() {
		t.Fatalf("lost business message: %s", got)
	}
}
