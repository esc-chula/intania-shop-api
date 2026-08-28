package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

// projectBodyLimit caps a project payload, which only ever carries a name, a
// description, and two dates.
const projectBodyLimit = 1 << 20

// ProjectMutator creates, updates, and deletes projects for the HTTP layer.
type ProjectMutator interface {
	Create(context.Context, models.ProjectInput) (models.Project, error)
	Update(context.Context, int64, models.ProjectInput) (models.Project, error)
	Delete(context.Context, int64) error
}

// ProjectAdminHandler exposes the admin project mutation endpoints.
type ProjectAdminHandler struct{ projects ProjectMutator }

// NewProjectAdminHandler constructs the project mutation HTTP adapter.
func NewProjectAdminHandler(projects ProjectMutator) *ProjectAdminHandler {
	return &ProjectAdminHandler{projects: projects}
}

// Register mounts the project mutation routes on the router.
func (handler *ProjectAdminHandler) Register(router chi.Router) {
	router.Post("/projects", handler.create)
	router.Put("/projects/{project_id}", handler.update)
	router.Delete("/projects/{project_id}", handler.delete)
}

func (handler *ProjectAdminHandler) create(writer http.ResponseWriter, request *http.Request) {
	input, ok := decodeProjectInput(writer, request)
	if !ok {
		return
	}
	project, err := handler.projects.Create(request.Context(), input)
	if err != nil {
		writeProjectMutationError(writer, request, err, "Unable to create project")
		return
	}
	writeSuccess(writer, http.StatusCreated, project)
}

func (handler *ProjectAdminHandler) update(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}
	input, ok := decodeProjectInput(writer, request)
	if !ok {
		return
	}
	project, err := handler.projects.Update(request.Context(), projectID, input)
	if err != nil {
		writeProjectMutationError(writer, request, err, "Unable to update project")
		return
	}
	writeSuccess(writer, http.StatusOK, project)
}

func (handler *ProjectAdminHandler) delete(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}
	if err := handler.projects.Delete(request.Context(), projectID); err != nil {
		writeProjectMutationError(writer, request, err, "Unable to delete project")
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// projectPathID reads the project ID from the path. Only the parse is a
// transport concern; the valid range belongs to the use case.
func projectPathID(writer http.ResponseWriter, request *http.Request) (int64, bool) {
	projectID, err := strconv.ParseInt(request.PathValue("project_id"), 10, 64)
	if err != nil {
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, "Invalid project ID")
		return 0, false
	}
	return projectID, true
}

// decodeProjectInput reads the mutation payload. Unknown fields are rejected so
// that a misspelled field is reported rather than silently ignored, and the
// decoder's own message is returned because it names the offending value.
func decodeProjectInput(writer http.ResponseWriter, request *http.Request) (models.ProjectInput, bool) {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, projectBodyLimit))
	decoder.DisallowUnknownFields()

	var input models.ProjectInput
	if err := decoder.Decode(&input); err != nil {
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation,
			fmt.Sprintf("Invalid request body: %s", err))
		return models.ProjectInput{}, false
	}
	return input, true
}

// writeProjectMutationError maps a use case failure onto the contract's status
// codes and error codes, falling back to an internal error.
func writeProjectMutationError(writer http.ResponseWriter, request *http.Request, err error, fallbackMessage string) {
	var validation usecases.ProjectValidationError
	switch {
	case errors.As(err, &validation):
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, validation.Message)
	case errors.Is(err, usecases.ErrInvalidProjectID):
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, "Invalid project ID")
	case errors.Is(err, repositories.ErrProjectNotFound):
		writeProjectError(writer, request, http.StatusNotFound, models.ProjectErrorNotFound, "Project not found")
	case errors.Is(err, repositories.ErrProjectHasOrders):
		writeProjectError(writer, request, http.StatusConflict, models.ProjectErrorHasOrders,
			"Project has orders and cannot be deleted")
	default:
		writeProjectError(writer, request, http.StatusInternalServerError, models.ProjectErrorInternal, fallbackMessage)
	}
}
