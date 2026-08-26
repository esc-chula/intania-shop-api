package models

import (
	"testing"
	"time"
)

func TestProjectStatusForUsesInclusiveCalendarRange(t *testing.T) {
	t.Parallel()

	start := mustParseDate(t, "2026-08-27")
	end := mustParseDate(t, "2026-08-29")

	tests := []struct {
		name  string
		today string
		want  ProjectStatus
	}{
		{name: "before start", today: "2026-08-26", want: ProjectStatusNotStarted},
		{name: "on start", today: "2026-08-27", want: ProjectStatusActive},
		{name: "between dates", today: "2026-08-28", want: ProjectStatusActive},
		{name: "on end", today: "2026-08-29", want: ProjectStatusActive},
		{name: "after end", today: "2026-08-30", want: ProjectStatusCompleted},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			today := mustParseDate(t, test.today)
			if got := ProjectStatusFor(start, end, today); got != test.want {
				t.Errorf("ProjectStatusFor(%s) = %q, want %q", test.today, got, test.want)
			}
		})
	}
}

func TestProjectStatusForOneDayProjectIsActiveOnItsDate(t *testing.T) {
	t.Parallel()

	projectDate := mustParseDate(t, "2026-08-27")
	today := NewDate(time.Date(2026, time.August, 27, 23, 59, 59, 0, time.FixedZone("test", 7*60*60)))

	if got := ProjectStatusFor(projectDate, projectDate, today); got != ProjectStatusActive {
		t.Errorf("ProjectStatusFor(one-day project) = %q, want %q", got, ProjectStatusActive)
	}
}

func mustParseDate(t *testing.T, value string) Date {
	t.Helper()

	date, err := ParseDate(value)
	if err != nil {
		t.Fatalf("parse test date %q: %v", value, err)
	}
	return date
}
