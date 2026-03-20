package entity

import (
	"strconv"
	"testing"

	videovo "f_ccg_video_back_ddd/internal/domain/video/valueobject"
)

func TestChunkSetIgnoresOutOfRangeExtras(t *testing.T) {
	valid := func(s string) error { _, err := strconv.Atoi(s); return err }
	if err := videovo.ValidateChunkSet([]string{"0", "1", "900"}, 2, valid); err != nil {
		t.Fatal(err)
	}
	if err := videovo.ValidateChunkSet([]string{"0", "900"}, 2, valid); err == nil {
		t.Fatal("missing required chunk accepted")
	}
}
