package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

// ProjectQuerier serves project queries to the HTTP layer.
type ProjectQuerier interface {
	List(context.Context, string, string, int32, int32) (models.ProjectListResponse, error)
	Detail(context.Context, int64) (models.Project, error)
}

// ProjectHandler exposes the admin project query endpoints.
type ProjectHandler struct{ projects ProjectQuerier }

// NewProjectHandler constructs the project HTTP adapter.
func NewProjectHandler(projects ProjectQuerier) *ProjectHandler {
	return &ProjectHandler{projects: projects}
}

// Register mounts the project routes on the router.
func (handler *ProjectHandler) Register(router chi.Router) {
	router.Get("/projects", handler.list)
	router.Get("/projects/{project_id}", handler.detail)
}

func (handler *ProjectHandler) list(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	data, err := handler.projects.List(request.Context(), query.Get("name"), query.Get("status"),
		queryInt(request, "page", 1), queryInt(request, "page_size", 10))
	if err != nil {
		writeProjectQueryError(writer, request, err, "Unable to list projects")
		return
	}
	writeSuccess(writer, http.StatusOK, data)
}

func (handler *ProjectHandler) detail(writer http.ResponseWriter, request *http.Request) {
	// Only the parse is a transport concern; the valid range belongs to the use case.
	projectID, err := strconv.ParseInt(request.PathValue("project_id"), 10, 64)
	if err != nil {
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, "Invalid project ID")
		return
	}
	project, err := handler.projects.Detail(request.Context(), projectID)
	if err != nil {
		writeProjectQueryError(writer, request, err, "Unable to get project")
		return
	}
	writeSuccess(writer, http.StatusOK, project)
}

// writeProjectQueryError maps a use case failure onto the contract's status
// codes and error codes, falling back to an internal error.
func writeProjectQueryError(writer http.ResponseWriter, request *http.Request, err error, fallbackMessage string) {
	switch {
	case errors.Is(err, repositories.ErrProjectNotFound):
		writeProjectError(writer, request, http.StatusNotFound, models.ProjectErrorNotFound, "Project not found")
	case errors.Is(err, usecases.ErrInvalidProjectStatus):
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, "Invalid project status")
	case errors.Is(err, usecases.ErrProjectNameTooLong):
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, "Project name filter is too long")
	case errors.Is(err, usecases.ErrInvalidProjectID):
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, "Invalid project ID")
	default:
		writeProjectError(writer, request, http.StatusInternalServerError, models.ProjectErrorInternal, fallbackMessage)
	}
}
