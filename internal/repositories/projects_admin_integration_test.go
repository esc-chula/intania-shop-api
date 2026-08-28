//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
)

// mutationInput builds a validated create or update payload.
func mutationInput(t *testing.T, name string, description *string, startDate, endDate string) models.ProjectInput {
	t.Helper()

	start, end := projectDate(t, startDate), projectDate(t, endDate)
	return models.ProjectInput{Name: &name, Description: description, StartDate: &start, EndDate: &end}
}

// A created project is described exactly like a listed one, so its status is
// derived and its order count starts at zero.
func TestProjectRepositoryCreate(t *testing.T) {
	pool := projectTestPool(t)
	seedProjects(t, pool)
	repository := repositories.NewProjectRepository(pool)
	today := projectDate(t, projectToday)
	description := "Main merchandise booth"

	tests := []struct {
		name               string
		description        *string
		startDate, endDate string
		wantStatus         models.ProjectStatus
	}{
		{name: "single day", description: &description, startDate: projectToday, endDate: projectToday, wantStatus: models.ProjectStatusActive},
		{name: "multi day ahead", description: nil, startDate: "2026-09-05", endDate: "2026-09-07", wantStatus: models.ProjectStatusNotStarted},
		{name: "multi day past", description: nil, startDate: "2026-08-01", endDate: "2026-08-03", wantStatus: models.ProjectStatusCompleted},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project, err := repository.Create(context.Background(), today,
				mutationInput(t, test.name, test.description, test.startDate, test.endDate))
			if err != nil {
				t.Fatalf("create project: %v", err)
			}
			if project.ProjectID <= 0 {
				t.Errorf("project_id=%d, want a generated identifier", project.ProjectID)
			}
			if project.Name != test.name {
				t.Errorf("name=%q want %q", project.Name, test.name)
			}
			if project.StartDate.String() != test.startDate || project.EndDate.String() != test.endDate {
				t.Errorf("dates=%s..%s want %s..%s", project.StartDate, project.EndDate, test.startDate, test.endDate)
			}
			if project.Status != test.wantStatus {
				t.Errorf("status=%q want %q", project.Status, test.wantStatus)
			}
			if project.OrderCount != 0 {
				t.Errorf("order_count=%d, want 0 for a new project", project.OrderCount)
			}
			if test.description == nil && project.Description != nil {
				t.Errorf("description=%q, want null", *project.Description)
			}
			if test.description != nil && (project.Description == nil || *project.Description != *test.description) {
				t.Errorf("description=%v want %q", project.Description, *test.description)
			}
		})
	}
}

// An update replaces every editable field and advances updated_at, which the
// table has no trigger to maintain.
func TestProjectRepositoryUpdate(t *testing.T) {
	pool := projectTestPool(t)
	seedProjects(t, pool)
	repository := repositories.NewProjectRepository(pool)
	today := projectDate(t, projectToday)
	ctx := context.Background()

	original, err := repository.Create(ctx, today, mutationInput(t, "Engineering Fair", nil, "2026-09-05", "2026-09-07"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	description := "Renamed booth"
	updated, err := repository.Update(ctx, today, original.ProjectID,
		mutationInput(t, "Engineering Fair 2026", &description, projectToday, projectToday))
	if err != nil {
		t.Fatalf("update project: %v", err)
	}

	if updated.ProjectID != original.ProjectID {
		t.Errorf("project_id=%d want %d", updated.ProjectID, original.ProjectID)
	}
	if updated.Name != "Engineering Fair 2026" {
		t.Errorf("name=%q want %q", updated.Name, "Engineering Fair 2026")
	}
	if updated.Description == nil || *updated.Description != description {
		t.Errorf("description=%v want %q", updated.Description, description)
	}
	if updated.StartDate.String() != projectToday || updated.EndDate.String() != projectToday {
		t.Errorf("dates=%s..%s want %s twice", updated.StartDate, updated.EndDate, projectToday)
	}
	// The new dates now contain today, so the derived status has to follow.
	if updated.Status != models.ProjectStatusActive {
		t.Errorf("status=%q want %q", updated.Status, models.ProjectStatusActive)
	}
	if !updated.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("created_at=%s want it unchanged at %s", updated.CreatedAt, original.CreatedAt)
	}
	if !updated.UpdatedAt.After(original.UpdatedAt) {
		t.Errorf("updated_at=%s, want it after %s", updated.UpdatedAt, original.UpdatedAt)
	}
}

func TestProjectRepositoryMutatesOnlyExistingProjects(t *testing.T) {
	pool := projectTestPool(t)
	ids := seedProjects(t, pool)
	repository := repositories.NewProjectRepository(pool)
	today := projectDate(t, projectToday)
	ctx := context.Background()

	// An identifier past every seeded row cannot match a project.
	missing := ids["Alumni Homecoming"] + 1000
	if _, err := repository.Update(ctx, today, missing, mutationInput(t, "Ghost", nil, projectToday, projectToday)); !errors.Is(err, repositories.ErrProjectNotFound) {
		t.Errorf("update err=%v, want ErrProjectNotFound", err)
	}
	if err := repository.Delete(ctx, missing); !errors.Is(err, repositories.ErrProjectNotFound) {
		t.Errorf("delete err=%v, want ErrProjectNotFound", err)
	}
}

// Delete is a hard delete: the row is gone rather than flagged.
func TestProjectRepositoryDeleteRemovesTheRow(t *testing.T) {
	pool := projectTestPool(t)
	ids := seedProjects(t, pool)
	repository := repositories.NewProjectRepository(pool)
	today := projectDate(t, projectToday)
	ctx := context.Background()

	projectID := ids["Engineering Fair"]
	if err := repository.Delete(ctx, projectID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	if _, err := repository.Detail(ctx, today, projectID); !errors.Is(err, repositories.ErrProjectNotFound) {
		t.Errorf("detail err=%v, want ErrProjectNotFound after a hard delete", err)
	}
	if err := repository.Delete(ctx, projectID); !errors.Is(err, repositories.ErrProjectNotFound) {
		t.Errorf("second delete err=%v, want ErrProjectNotFound", err)
	}
}

// Orders reference projects with ON DELETE RESTRICT, so a project with sales
// has to be reported as a conflict rather than as an unexpected failure.
func TestProjectRepositoryDeleteRejectsProjectsWithOrders(t *testing.T) {
	pool := projectTestPool(t)
	ids := seedProjects(t, pool)
	repository := repositories.NewProjectRepository(pool)
	ctx := context.Background()

	// "freshy night" is seeded with two orders.
	if err := repository.Delete(ctx, ids["freshy night"]); !errors.Is(err, repositories.ErrProjectHasOrders) {
		t.Fatalf("delete err=%v, want ErrProjectHasOrders", err)
	}
	project, err := repository.Detail(ctx, projectDate(t, projectToday), ids["freshy night"])
	if err != nil {
		t.Fatalf("get project after a rejected delete: %v", err)
	}
	if project.OrderCount != 2 {
		t.Errorf("order_count=%d want 2", project.OrderCount)
	}
}
