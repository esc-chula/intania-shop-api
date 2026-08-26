package models

import "time"

// Date represents a calendar date without a time of day.
type Date struct {
	time.Time
}

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
