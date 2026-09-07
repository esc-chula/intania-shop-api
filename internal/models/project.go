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
	// ProjectErrorConflict reports a state or reference that prevents a change.
	ProjectErrorConflict ProjectAPIErrorCode = "PROJECT_CONFLICT"
	// ProjectErrorInternal reports an unexpected server failure.
	ProjectErrorInternal ProjectAPIErrorCode = "INTERNAL_ERROR"
)

const ProjectNameMaxLength = 150
const ProjectDescriptionMaxLength = 2000
const ProjectErrorHasOrders ProjectAPIErrorCode = "PROJECT_HAS_ORDERS"

type ProjectFilter struct {
	Name   string
	Status ProjectStatus
}

// ProjectListResponse is the paginated project listing response.
type ProjectListResponse struct {
	Projects   []Project `json:"projects"`
	Total      int64     `json:"total"`
	Page       int32     `json:"page"`
	PageSize   int32     `json:"page_size"`
	TotalPages int32     `json:"total_pages"`
}

type ProjectInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	StartDate   *Date   `json:"start_date"`
	EndDate     *Date   `json:"end_date"`
}

type ProjectProductAssignmentInput struct {
	ProductID    int64  `json:"product_id"`
	VariantID    *int64 `json:"variant_id"`
	ProjectPrice string `json:"project_price"`
}

// ReplaceProjectProductsRequest uses a pointer so a missing items property is
// distinguishable from an explicit empty array, which clears the selection.
type ReplaceProjectProductsRequest struct {
	Items *[]ProjectProductAssignmentInput `json:"items"`
}

// ProductNameFilterMaxLength and ProductCategoryFilterMaxLength match the
// products columns the candidate filters compare against.
const ProductNameFilterMaxLength = 150
const ProductCategoryFilterMaxLength = 100

// ProjectProductFilter narrows the product candidate list. Both filters are
// optional and combine.
type ProjectProductFilter struct {
	Name     string
	Category string
}

// ProjectSellableItem is one purchasable product or product variant offered to
// the project picker, with the stock read live from the catalogue and the
// selection state read from the project's assignments. ProjectPrice is null
// exactly when the item is not selected.
type ProjectSellableItem struct {
	ProductID     int64   `json:"product_id"`
	VariantID     *int64  `json:"variant_id"`
	Size          *string `json:"size"`
	Color         *string `json:"color"`
	StockQuantity int32   `json:"stock_quantity"`
	DefaultPrice  string  `json:"default_price"`
	Selected      bool    `json:"selected"`
	ProjectPrice  *string `json:"project_price"`
}

// ProjectProductCandidate is a product grouped with its sellable items. A
// product without variants carries exactly one item, whose VariantID is null.
type ProjectProductCandidate struct {
	ProductID     int64                 `json:"product_id"`
	Name          string                `json:"name"`
	Category      *string               `json:"category"`
	ImageURL      *string               `json:"image_url"`
	SellableItems []ProjectSellableItem `json:"sellable_items"`
}

// ProjectProductCandidateListResponse is the paginated candidate response.
// Pagination counts products rather than sellable items.
type ProjectProductCandidateListResponse struct {
	Products   []ProjectProductCandidate `json:"products"`
	Total      int64                     `json:"total"`
	Page       int32                     `json:"page"`
	PageSize   int32                     `json:"page_size"`
	TotalPages int32                     `json:"total_pages"`
}

// ProjectProductAssignment is one sellable item that is in the project, priced
// at the project price.
type ProjectProductAssignment struct {
	ProductID     int64   `json:"product_id"`
	VariantID     *int64  `json:"variant_id"`
	ProductName   string  `json:"product_name"`
	Category      *string `json:"category"`
	ImageURL      *string `json:"image_url"`
	Size          *string `json:"size"`
	Color         *string `json:"color"`
	StockQuantity int32   `json:"stock_quantity"`
	ProjectPrice  string  `json:"project_price"`
}

type ProjectProductsResponse struct {
	Items []ProjectProductAssignment `json:"items"`
}
