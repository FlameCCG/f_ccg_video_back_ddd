package contentops

import (
	"testing"
	"time"
)

func TestStatsWindowsKeepCalendarBoundariesAcrossYear(t *testing.T) {
	zone := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 1, 2, 17, 0, 0, 0, zone)
	windows := (StatsPolicy{}).Windows(now)
	if len(windows.Daily) != 7 || len(windows.Monthly) != 12 {
		t.Fatalf("window lengths: %+v", windows)
	}
	if got := windows.Daily[0].Start.Format("2006-01-02"); got != "2025-12-27" {
		t.Fatal(got)
	}
	if got := windows.Monthly[0].Start.Format("2006-01-02"); got != "2025-02-01" {
		t.Fatal(got)
	}
	if windows.Daily[6] != windows.Today || windows.Today.End.Day() != 3 || windows.Today.Start.Hour() != 0 {
		t.Fatalf("today differs from last daily window: %+v", windows)
	}
	for _, periods := range [][]StatsPeriod{windows.Daily, windows.Monthly} {
		for i, period := range periods {
			if !period.Start.Before(period.End) || period.Start.Location() != zone || i > 0 && period.Start != periods[i-1].End {
				t.Fatalf("non-adjacent calendar periods: %+v", periods)
			}
		}
	}
}
