package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

// ProjectService applies query rules to project reads.
type ProjectService struct {
	projects ProjectReader
	now      func() time.Time
}

// NewProjectService constructs the project query use case.
func NewProjectService(projects ProjectReader) *ProjectService {
	return &ProjectService{projects: projects, now: time.Now}
}

// List returns a filtered, paginated page of projects. Filters are applied
// before pagination so that the reported total counts only matching projects.
func (service *ProjectService) List(ctx context.Context, name, status string, page, pageSize int32) (models.ProjectListResponse, error) {
	filter, err := buildProjectFilter(name, status)
	if err != nil {
		return models.ProjectListResponse{}, err
	}

	page, pageSize = normalizePage(page, pageSize)
	projects, total, err := service.projects.List(ctx, service.today(), filter, (page-1)*pageSize, pageSize)
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
	project, err := service.projects.Detail(ctx, service.today(), projectID)
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
		if len(trimmed) > models.ProjectNameMaxLength {
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
