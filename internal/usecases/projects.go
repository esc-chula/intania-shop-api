package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

// Query rules are enforced here rather than in the HTTP layer, so that the
// transport only has to map these errors onto status codes.
var (
	// ErrInvalidProjectStatus reports a status filter outside the enum.
	ErrInvalidProjectStatus = errors.New("invalid project status")
	// ErrProjectNameTooLong reports a name filter longer than a project name.
	ErrProjectNameTooLong = errors.New("project name filter is too long")
	// ErrInvalidProjectID reports a project ID outside the valid range.
	ErrInvalidProjectID = errors.New("project ID must be positive")
)

// ProjectReader reads projects with their derived status and order count.
type ProjectReader interface {
	List(context.Context, models.Date, models.ProjectFilter, int32, int32) ([]models.Project, int64, error)
	Detail(context.Context, models.Date, int64) (models.Project, error)
}

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

// ProjectService applies query rules to project reads.
type ProjectService struct {
	reader ProjectReader
	writer ProjectWriter
	now    func() time.Time
}

// NewProjectService constructs the project use case.
func NewProjectService(reader ProjectReader, writer ProjectWriter) *ProjectService {
	return &ProjectService{reader: reader, writer: writer, now: time.Now}
}

// List returns a filtered, paginated page of projects. Filters are applied
// before pagination so that the reported total counts only matching projects.
func (service *ProjectService) List(ctx context.Context, name, status string, page, pageSize int32) (models.ProjectListResponse, error) {
	filter, err := buildProjectFilter(name, status)
	if err != nil {
		return models.ProjectListResponse{}, err
	}

	page, pageSize = normalizePage(page, pageSize)
	projects, total, err := service.reader.List(ctx, service.today(), filter, (page-1)*pageSize, pageSize)
	if err != nil {
		return models.ProjectListResponse{}, fmt.Errorf("list projects: %w", err)
	}
	return models.ProjectListResponse{
		Projects: projects, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages(total, pageSize),
	}, nil
}

// Detail returns a single project.
func (service *ProjectService) Detail(ctx context.Context, projectID int64) (models.Project, error) {
	if projectID <= 0 {
		return models.Project{}, ErrInvalidProjectID
	}
	project, err := service.reader.Detail(ctx, service.today(), projectID)
	if err != nil {
		return models.Project{}, fmt.Errorf("get project detail: %w", err)
	}
	return project, nil
}

func (service *ProjectService) today() models.Date {
	return models.TodayInBangkok(service.now())
}

func buildProjectFilter(name, status string) (models.ProjectFilter, error) {
	var filter models.ProjectFilter
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		if utf8.RuneCountInString(trimmed) > models.ProjectNameMaxLength {
			return models.ProjectFilter{}, ErrProjectNameTooLong
		}
		filter.Name = &trimmed
	}
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		parsed, err := models.ParseProjectStatus(trimmed)
		if err != nil {
			return models.ProjectFilter{}, fmt.Errorf("%w: %q", ErrInvalidProjectStatus, trimmed)
		}
		filter.Status = &parsed
	}
	return filter, nil
}

// Create persists a new project and returns it with its derived status.
func (service *ProjectService) Create(ctx context.Context, input models.ProjectInput) (models.Project, error) {
	validated, err := validateProjectInput(input)
	if err != nil {
		return models.Project{}, err
	}
	return service.writer.Create(ctx, service.today(), validated)
}

// Update replaces the editable fields of an existing project. Every field is
// required, so an update is a complete replacement rather than a patch.
func (service *ProjectService) Update(ctx context.Context, projectID int64, input models.ProjectInput) (models.Project, error) {
	if projectID <= 0 {
		return models.Project{}, ErrInvalidProjectID
	}
	validated, err := validateProjectInput(input)
	if err != nil {
		return models.Project{}, err
	}
	return service.writer.Update(ctx, service.today(), projectID, validated)
}

// Delete permanently removes a project.
func (service *ProjectService) Delete(ctx context.Context, projectID int64) error {
	if projectID <= 0 {
		return ErrInvalidProjectID
	}
	return service.writer.Delete(ctx, projectID)
}

// validateProjectInput enforces the create and update rules and returns the
// normalized payload that is persisted. Lengths are counted in characters
// because that is how the projects table measures its columns.
func validateProjectInput(input models.ProjectInput) (models.ProjectInput, error) {
	if input.Name == nil {
		return models.ProjectInput{}, ProjectValidationError{Message: "Project name is required"}
	}
	name := strings.TrimSpace(*input.Name)
	if name == "" {
		return models.ProjectInput{}, ProjectValidationError{Message: "Project name must not be empty"}
	}
	if utf8.RuneCountInString(name) > models.ProjectNameMaxLength {
		return models.ProjectInput{}, ProjectValidationError{
			Message: fmt.Sprintf("Project name must be at most %d characters", models.ProjectNameMaxLength),
		}
	}
	input.Name = &name

	if input.Description != nil && utf8.RuneCountInString(*input.Description) > models.ProjectDescriptionMaxLength {
		return models.ProjectInput{}, ProjectValidationError{
			Message: fmt.Sprintf("Project description must be at most %d characters", models.ProjectDescriptionMaxLength),
		}
	}
	if input.StartDate == nil {
		return models.ProjectInput{}, ProjectValidationError{Message: "Project start date is required"}
	}
	if input.EndDate == nil {
		return models.ProjectInput{}, ProjectValidationError{Message: "Project end date is required"}
	}
	if input.EndDate.Before(input.StartDate.Time) {
		return models.ProjectInput{}, ProjectValidationError{Message: "Project end date must not be earlier than the start date"}
	}
	return input, nil
}
