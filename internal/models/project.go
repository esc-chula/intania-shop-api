package models

import (
	"fmt"
	"time"
)

// ProjectStatus is the derived lifecycle state of a project.
type ProjectStatus string

const (
	// ProjectStatusNotStarted indicates that the project has not begun yet.
	ProjectStatusNotStarted ProjectStatus = "NOT_STARTED"
	// ProjectStatusActive indicates that the project is currently in progress.
	ProjectStatusActive ProjectStatus = "ACTIVE"
	// ProjectStatusCompleted indicates that the project has ended.
	ProjectStatusCompleted ProjectStatus = "COMPLETED"
)

// ProjectStatusFor returns the project status for a calendar date.
func ProjectStatusFor(startDate Date, endDate Date, today Date) ProjectStatus {
	// assuming that endDate >= startDate

	if today.Before(startDate.Time) {
		return ProjectStatusNotStarted
	}

	if today.After(endDate.Time) {
		return ProjectStatusCompleted
	}

	return ProjectStatusActive
}

// Project is a time-bounded project with status and order count derived from its dates and linked orders.
type Project struct {
	ProjectID   int64         `json:"project_id"`
	Name        string        `json:"name"`
	Description *string       `json:"description"`
	StartDate   Date          `json:"start_date"`
	EndDate     Date          `json:"end_date"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Status      ProjectStatus `json:"status"`
	OrderCount  int64         `json:"order_count"`
}

// bangkok is the project calendar timezone used to derive status.
var bangkok = time.FixedZone("Asia/Bangkok", 7*60*60)

// TodayInBangkok returns the current calendar date in the project timezone.
func TodayInBangkok(now time.Time) Date {
	return NewDate(now.In(bangkok))
}

// ParseProjectStatus converts a client-supplied status filter value.
func ParseProjectStatus(value string) (ProjectStatus, error) {
	switch ProjectStatus(value) {
	case ProjectStatusNotStarted, ProjectStatusActive, ProjectStatusCompleted:
		return ProjectStatus(value), nil
	default:
		return "", fmt.Errorf("invalid project status %q", value)
	}
}

// ProjectAPIErrorCode is the machine-readable code carried by every
// Project/POS error response.
type ProjectAPIErrorCode string

const (
	// ProjectErrorValidation reports a rejected request parameter.
	ProjectErrorValidation ProjectAPIErrorCode = "VALIDATION_ERROR"
	// ProjectErrorNotFound reports a project that does not exist.
	ProjectErrorNotFound ProjectAPIErrorCode = "PROJECT_NOT_FOUND"
	// ProjectErrorInternal reports an unexpected server failure.
	ProjectErrorInternal ProjectAPIErrorCode = "INTERNAL_ERROR"
)

// ProjectNameMaxLength is the longest accepted project name, matching the
// projects.name column and the shared contract.
const ProjectNameMaxLength = 150

// ProjectFilter narrows a project listing before pagination is applied.
type ProjectFilter struct {
	Name   *string
	Status *ProjectStatus
}

// ProjectListResponse is the paginated project listing response.
type ProjectListResponse struct {
	Projects   []Project `json:"projects"`
	Total      int64     `json:"total"`
	Page       int32     `json:"page"`
	PageSize   int32     `json:"page_size"`
	TotalPages int32     `json:"total_pages"`
}
