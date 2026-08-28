package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/go-chi/chi/v5"
)

type projectServiceStub struct {
	name, status   string
	page, pageSize int32
	detailErr      error
}

func (stub *projectServiceStub) List(_ context.Context, name, status string, page, pageSize int32) (models.ProjectListResponse, error) {
	stub.name, stub.status, stub.page, stub.pageSize = name, status, page, pageSize
	return models.ProjectListResponse{
		Projects: []models.Project{{ProjectID: 7, Name: "Engineering Fair", Status: models.ProjectStatusActive, OrderCount: 3}},
		Total:    1, Page: page, PageSize: pageSize, TotalPages: 1,
	}, nil
}

func (stub *projectServiceStub) Detail(_ context.Context, projectID int64) (models.Project, error) {
	if stub.detailErr != nil {
		return models.Project{}, stub.detailErr
	}
	return models.Project{ProjectID: projectID, Name: "Engineering Fair", OrderCount: 3}, nil
}

func projectRouter(stub *projectServiceStub) chi.Router {
	router := chi.NewRouter()
	NewProjectHandler(stub).Register(router)
	return router
}

func TestProjectHandlerListPassesQueryParameters(t *testing.T) {
	stub := &projectServiceStub{}
	recorder := httptest.NewRecorder()
	projectRouter(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/projects?name=fair&status=ACTIVE&page=2&page_size=5", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", recorder.Code, http.StatusOK)
	}
	if stub.name != "fair" || stub.status != "ACTIVE" || stub.page != 2 || stub.pageSize != 5 {
		t.Errorf("service received name=%q status=%q page=%d page_size=%d", stub.name, stub.status, stub.page, stub.pageSize)
	}

	var body struct {
		Success bool                       `json:"success"`
		Data    models.ProjectListResponse `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || len(body.Data.Projects) != 1 || body.Data.Projects[0].OrderCount != 3 {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestProjectHandlerListDefaultsPagination(t *testing.T) {
	stub := &projectServiceStub{}
	projectRouter(stub).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/projects", nil))

	if stub.page != 1 || stub.pageSize != 10 {
		t.Errorf("page=%d page_size=%d, want 1 and 10", stub.page, stub.pageSize)
	}
}

func TestProjectHandlerListRejectsUnknownStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	projectRouter(&projectServiceStub{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/projects?status=RUNNING", nil))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestProjectHandlerDetail(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		detailErr error
		want      int
	}{
		{name: "existing project", target: "/projects/7", want: http.StatusOK},
		{name: "non-numeric ID", target: "/projects/abc", want: http.StatusBadRequest},
		{name: "non-positive ID", target: "/projects/0", want: http.StatusBadRequest},
		{name: "missing project", target: "/projects/7", detailErr: repositories.ErrProjectNotFound, want: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			projectRouter(&projectServiceStub{detailErr: test.detailErr}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.target, nil))
			if recorder.Code != test.want {
				t.Fatalf("status=%d want %d", recorder.Code, test.want)
			}
		})
	}
}
