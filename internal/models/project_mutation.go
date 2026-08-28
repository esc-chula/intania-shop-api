package models

// ProjectDescriptionMaxLength is the longest accepted project description,
// matching the shared contract.
const ProjectDescriptionMaxLength = 2000

// ProjectErrorHasOrders reports a project that cannot be deleted because sales
// reference it.
const ProjectErrorHasOrders ProjectAPIErrorCode = "PROJECT_HAS_ORDERS"

// ProjectInput is the create/update project payload. Pointers distinguish an
// omitted field from an empty one, so that a missing required date is reported
// as a validation error rather than silently read as the zero date.
type ProjectInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	StartDate   *Date   `json:"start_date"`
	EndDate     *Date   `json:"end_date"`
}
