package entity_test

import (
	"strings"
	"testing"
	"time"

	videoentity "f_ccg_video_back_ddd/internal/domain/video/entity"
)

func TestParseDanmuMonthRange(t *testing.T) {
	t.Parallel()

	start, end, err := videoentity.ParseDanmuMonthRange("2026-10")
	if err != nil {
		t.Fatal(err)
	}

	if start.Year() != 2026 || start.Month() != time.October || start.Day() != 1 {
		t.Fatalf("start = %s", start)
	}

	if !end.Equal(start.AddDate(0, 1, 0)) {
		t.Fatalf("end = %s", end)
	}

	_, _, err = videoentity.ParseDanmuMonthRange("2026-13")
	if err == nil || !strings.Contains(err.Error(), "YYYY-MM") {
		t.Fatalf("invalid month err = %v", err)
	}
}

func TestParseDanmuInstantRange(t *testing.T) {
	t.Parallel()

	start, end, err := videoentity.ParseDanmuInstantRange(
		"2026-10-01T07:00:00.000Z",
		"2026-11-01T07:00:00.000Z",
	)
	if err != nil {
		t.Fatal(err)
	}

	if !end.After(start) {
		t.Fatalf("end = %s", end)
	}

	_, _, err = videoentity.ParseDanmuInstantRange("2026-11-01T00:00:00Z", "2026-10-01T00:00:00Z")
	if err == nil {
		t.Fatal("reversed range should fail")
	}
}
