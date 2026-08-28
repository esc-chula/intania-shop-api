package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

type projectMutatorStub struct {
	created   models.ProjectInput
	updatedID int64
	deletedID int64
	err       error
}

func (stub *projectMutatorStub) Create(_ context.Context, input models.ProjectInput) (models.Project, error) {
	stub.created = input
	if stub.err != nil {
		return models.Project{}, stub.err
	}
	return models.Project{ProjectID: 7, Name: *input.Name, Status: models.ProjectStatusNotStarted}, nil
}

func (stub *projectMutatorStub) Update(_ context.Context, projectID int64, input models.ProjectInput) (models.Project, error) {
	stub.updatedID = projectID
	if stub.err != nil {
		return models.Project{}, stub.err
	}
	return models.Project{ProjectID: projectID, Name: *input.Name}, nil
}

func (stub *projectMutatorStub) Delete(_ context.Context, projectID int64) error {
	stub.deletedID = projectID
	return stub.err
}

func projectAdminRouter(stub *projectMutatorStub) chi.Router {
	router := chi.NewRouter()
	// RequestID supplies the identifier the error envelope has to carry.
	router.Use(middlewares.RequestID(slog.New(slog.NewTextHandler(io.Discard, nil))))
	NewProjectAdminHandler(stub).Register(router)
	return router
}

const validProjectBody = `{"name":"Engineering Fair","description":"Main booth","start_date":"2026-09-05","end_date":"2026-09-07"}`

func projectRequest(method, target, body string) *http.Request {
	return httptest.NewRequest(method, target, strings.NewReader(body))
}

func TestProjectAdminHandlerCreate(t *testing.T) {
	stub := &projectMutatorStub{}
	recorder := httptest.NewRecorder()
	projectAdminRouter(stub).ServeHTTP(recorder, projectRequest(http.MethodPost, "/projects", validProjectBody))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d want %d", recorder.Code, http.StatusCreated)
	}
	if *stub.created.Name != "Engineering Fair" || stub.created.StartDate.String() != "2026-09-05" || stub.created.EndDate.String() != "2026-09-07" {
		t.Errorf("decoded payload is wrong: %+v", stub.created)
	}

	var body struct {
		Success bool           `json:"success"`
		Data    models.Project `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Data.ProjectID != 7 || body.Data.Status != models.ProjectStatusNotStarted {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestProjectAdminHandlerUpdate(t *testing.T) {
	stub := &projectMutatorStub{}
	recorder := httptest.NewRecorder()
	projectAdminRouter(stub).ServeHTTP(recorder, projectRequest(http.MethodPut, "/projects/7", validProjectBody))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", recorder.Code, http.StatusOK)
	}
	if stub.updatedID != 7 {
		t.Errorf("updated project %d, want 7", stub.updatedID)
	}
}

// A hard delete leaves nothing to return, so it answers with an empty 204.
func TestProjectAdminHandlerDelete(t *testing.T) {
	stub := &projectMutatorStub{}
	recorder := httptest.NewRecorder()
	projectAdminRouter(stub).ServeHTTP(recorder, projectRequest(http.MethodDelete, "/projects/7", ""))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d want %d", recorder.Code, http.StatusNoContent)
	}
	if stub.deletedID != 7 {
		t.Errorf("deleted project %d, want 7", stub.deletedID)
	}
	if recorder.Body.Len() != 0 {
		t.Errorf("body=%q, want it empty", recorder.Body.String())
	}
}

// A malformed body is rejected before the use case runs, and the message names
// what the client got wrong.
func TestProjectAdminHandlerRejectsMalformedBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "not JSON", body: `{`},
		{name: "unknown field", body: `{"name":"Fair","startDate":"2026-09-05"}`},
		{name: "unparsable date", body: `{"name":"Fair","start_date":"2026-13-01","end_date":"2026-09-07"}`},
		{name: "date is not a string", body: `{"name":"Fair","start_date":20260905,"end_date":"2026-09-07"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &projectMutatorStub{}
			recorder := httptest.NewRecorder()
			projectAdminRouter(stub).ServeHTTP(recorder, projectRequest(http.MethodPost, "/projects", test.body))
			assertProjectError(t, recorder, http.StatusBadRequest, models.ProjectErrorValidation)
			if stub.created.Name != nil {
				t.Error("a malformed payload reached the use case")
			}
		})
	}
}

// Validation rules live in the use case, so the handler has to pass its message
// through unchanged rather than restate it.
func TestProjectAdminHandlerReturnsTheValidationMessage(t *testing.T) {
	stub := &projectMutatorStub{err: usecases.ProjectValidationError{Message: "Project end date is required"}}
	recorder := httptest.NewRecorder()
	projectAdminRouter(stub).ServeHTTP(recorder, projectRequest(http.MethodPost, "/projects", validProjectBody))

	// The envelope assertion drains the recorder, so keep the payload first.
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	assertProjectError(t, recorder, http.StatusBadRequest, models.ProjectErrorValidation)
	if body.Error != "Project end date is required" {
		t.Errorf("error=%q, want the use case message", body.Error)
	}
}

func TestProjectAdminHandlerMapsUseCaseErrors(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		target   string
		err      error
		want     int
		wantCode models.ProjectAPIErrorCode
	}{
		{name: "non-numeric ID", method: http.MethodPut, target: "/projects/abc", want: http.StatusBadRequest, wantCode: models.ProjectErrorValidation},
		{name: "non-positive ID", method: http.MethodDelete, target: "/projects/0", err: usecases.ErrInvalidProjectID, want: http.StatusBadRequest, wantCode: models.ProjectErrorValidation},
		{name: "missing project on update", method: http.MethodPut, target: "/projects/7", err: repositories.ErrProjectNotFound, want: http.StatusNotFound, wantCode: models.ProjectErrorNotFound},
		{name: "missing project on delete", method: http.MethodDelete, target: "/projects/7", err: repositories.ErrProjectNotFound, want: http.StatusNotFound, wantCode: models.ProjectErrorNotFound},
		{name: "project has orders", method: http.MethodDelete, target: "/projects/7", err: repositories.ErrProjectHasOrders, want: http.StatusConflict, wantCode: models.ProjectErrorHasOrders},
		{name: "unexpected failure", method: http.MethodPost, target: "/projects", err: errors.New("boom"), want: http.StatusInternalServerError, wantCode: models.ProjectErrorInternal},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			projectAdminRouter(&projectMutatorStub{err: test.err}).
				ServeHTTP(recorder, projectRequest(test.method, test.target, validProjectBody))
			assertProjectError(t, recorder, test.want, test.wantCode)
		})
	}
}
