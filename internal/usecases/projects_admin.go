package usecases

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

// ProjectValidationError reports a rejected create or update payload. It
// carries the message written for the API client, so that the transport can
// pass it through instead of restating every rule.
type ProjectValidationError struct{ Message string }

// Error returns the client-facing validation message.
func (err ProjectValidationError) Error() string { return err.Message }

// ProjectWriter creates, updates, and deletes projects.
type ProjectWriter interface {
	Create(context.Context, models.Date, models.ProjectInput) (models.Project, error)
	Update(context.Context, models.Date, int64, models.ProjectInput) (models.Project, error)
	Delete(context.Context, int64) error
}

// ProjectAdminService applies mutation rules to project writes.
type ProjectAdminService struct {
	projects ProjectWriter
	now      func() time.Time
}

// NewProjectAdminService constructs the project mutation use case.
func NewProjectAdminService(projects ProjectWriter) *ProjectAdminService {
	return &ProjectAdminService{projects: projects, now: time.Now}
}

// Create persists a new project and returns it with its derived status.
func (service *ProjectAdminService) Create(ctx context.Context, input models.ProjectInput) (models.Project, error) {
	validated, err := validateProjectInput(input)
	if err != nil {
		return models.Project{}, err
	}
	project, err := service.projects.Create(ctx, service.today(), validated)
	if err != nil {
		return models.Project{}, fmt.Errorf("create project: %w", err)
	}
	return project, nil
}

// Update replaces the editable fields of an existing project. Every field is
// required, so an update is a complete replacement rather than a patch.
func (service *ProjectAdminService) Update(ctx context.Context, projectID int64, input models.ProjectInput) (models.Project, error) {
	if projectID <= 0 {
		return models.Project{}, ErrInvalidProjectID
	}
	validated, err := validateProjectInput(input)
	if err != nil {
		return models.Project{}, err
	}
	project, err := service.projects.Update(ctx, service.today(), projectID, validated)
	if err != nil {
		return models.Project{}, fmt.Errorf("update project: %w", err)
	}
	return project, nil
}

// Delete permanently removes a project.
func (service *ProjectAdminService) Delete(ctx context.Context, projectID int64) error {
	if projectID <= 0 {
		return ErrInvalidProjectID
	}
	if err := service.projects.Delete(ctx, projectID); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

func (service *ProjectAdminService) today() models.Date {
	return models.TodayInBangkok(service.now())
}

// validateProjectInput enforces the create and update rules and returns the
// normalized payload that is persisted. Lengths are counted in characters
// because that is how the projects table measures its columns.
func validateProjectInput(input models.ProjectInput) (models.ProjectInput, error) {
	if input.Name == nil {
		return input, ProjectValidationError{Message: "Project name is required"}
	}
	name := strings.TrimSpace(*input.Name)
	if name == "" {
		return input, ProjectValidationError{Message: "Project name must not be empty"}
	}
	if utf8.RuneCountInString(name) > models.ProjectNameMaxLength {
		return input, ProjectValidationError{
			Message: fmt.Sprintf("Project name must be at most %d characters", models.ProjectNameMaxLength),
		}
	}
	input.Name = &name

	if input.Description != nil && utf8.RuneCountInString(*input.Description) > models.ProjectDescriptionMaxLength {
		return input, ProjectValidationError{
			Message: fmt.Sprintf("Project description must be at most %d characters", models.ProjectDescriptionMaxLength),
		}
	}
	if input.StartDate == nil {
		return input, ProjectValidationError{Message: "Project start date is required"}
	}
	if input.EndDate == nil {
		return input, ProjectValidationError{Message: "Project end date is required"}
	}
	if input.EndDate.Before(input.StartDate.Time) {
		return input, ProjectValidationError{Message: "Project end date must not be earlier than the start date"}
	}
	return input, nil
}
