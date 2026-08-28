package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/go-chi/chi/v5"
)

// ProjectReader serves project queries to the HTTP layer.
type ProjectReader interface {
	List(context.Context, string, string, int32, int32) (models.ProjectListResponse, error)
	Detail(context.Context, int64) (models.Project, error)
}

// ProjectHandler exposes the admin project query endpoints.
type ProjectHandler struct{ projects ProjectReader }

// NewProjectHandler constructs the project HTTP adapter.
func NewProjectHandler(projects ProjectReader) *ProjectHandler {
	return &ProjectHandler{projects: projects}
}

// Register mounts the project routes on the router.
func (handler *ProjectHandler) Register(router chi.Router) {
	router.Get("/projects", handler.list)
	router.Get("/projects/{project_id}", handler.detail)
}

func (handler *ProjectHandler) list(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	status := query.Get("status")
	if status != "" {
		if _, err := models.ParseProjectStatus(status); err != nil {
			writeError(writer, http.StatusBadRequest, "Invalid project status")
			return
		}
	}
	data, err := handler.projects.List(request.Context(), query.Get("name"), status,
		queryInt(request, "page", 1), queryInt(request, "page_size", 10))
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "Unable to list projects")
		return
	}
	writeSuccess(writer, http.StatusOK, data)
}

func (handler *ProjectHandler) detail(writer http.ResponseWriter, request *http.Request) {
	projectID, err := strconv.ParseInt(request.PathValue("project_id"), 10, 64)
	if err != nil || projectID <= 0 {
		writeError(writer, http.StatusBadRequest, "Invalid project ID")
		return
	}
	project, err := handler.projects.Detail(request.Context(), projectID)
	if errors.Is(err, repositories.ErrProjectNotFound) {
		writeError(writer, http.StatusNotFound, "Project not found")
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "Unable to get project")
		return
	}
	writeSuccess(writer, http.StatusOK, project)
}
