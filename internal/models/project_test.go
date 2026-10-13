package models

import "testing"

func TestProjectEffectiveStatus(t *testing.T) {
	start, _ := ParseDate("2026-10-13")
	end, _ := ParseDate("2026-10-14")
	project := Project{StartDate: start, EndDate: end}
	for _, test := range []struct {
		date string
		want ProjectStatus
	}{
		{"2026-10-12", ProjectStatusNotStarted},
		{"2026-10-13", ProjectStatusActive},
		{"2026-10-14", ProjectStatusActive},
		{"2026-10-15", ProjectStatusCompleted},
	} {
		t.Run(test.date, func(t *testing.T) {
			today, _ := ParseDate(test.date)
			project.StatusOverride = nil
			if got := project.EffectiveStatus(today); got != test.want {
				t.Fatalf("automatic status = %s, want %s", got, test.want)
			}
			for _, status := range []ProjectStatus{ProjectStatusNotStarted, ProjectStatusActive, ProjectStatusCompleted} {
				project.StatusOverride = &status
				if got := project.EffectiveStatus(today); got != status {
					t.Fatalf("override status = %s, want %s", got, status)
				}
			}
		})
	}
}
