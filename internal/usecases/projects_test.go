package usecases

import (
	"errors"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

func TestValidateProjectStatusOverride(t *testing.T) {
	name := "Manual status"
	date, _ := models.ParseDate("2026-10-13")
	for _, value := range []string{"automatic", "NOT_STARTED", "ACTIVE", "COMPLETED", "", "active", "INVALID"} {
		t.Run(value, func(t *testing.T) {
			input := models.ProjectInput{Name: &name, StartDate: &date, EndDate: &date}
			if value != "automatic" {
				status := models.ProjectStatus(value)
				input.StatusOverride = &status
			}
			validated, err := validateProjectInput(input)
			invalid := value == "" || value == "active" || value == "INVALID"
			if invalid {
				var validation ProjectValidationError
				if !errors.As(err, &validation) {
					t.Fatalf("error = %v, want validation error", err)
				}
			} else if err != nil || validated.StatusOverride != input.StatusOverride {
				t.Fatalf("validated = %+v, error = %v", validated, err)
			}
		})
	}
}
