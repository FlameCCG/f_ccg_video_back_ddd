package video

import (
	"strconv"
	"testing"
)

func TestChunkSetIgnoresOutOfRangeExtras(t *testing.T) {
	valid := func(s string) error { _, err := strconv.Atoi(s); return err }
	if err := ValidateChunkSet([]string{"0", "1", "900"}, 2, valid); err != nil {
		t.Fatal(err)
	}
	if err := ValidateChunkSet([]string{"0", "900"}, 2, valid); err == nil {
		t.Fatal("missing required chunk accepted")
	}
}
