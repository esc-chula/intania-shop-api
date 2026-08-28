package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/handlers"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

type projectMutationStub struct{}

func (projectMutationStub) Create(_ context.Context, input models.ProjectInput) (models.Project, error) {
	return models.Project{ProjectID: 7, Name: *input.Name}, nil
}

func (projectMutationStub) Update(_ context.Context, projectID int64, _ models.ProjectInput) (models.Project, error) {
	return models.Project{ProjectID: projectID}, nil
}

func (projectMutationStub) Delete(context.Context, int64) error { return nil }

type projectQueryStub struct{}

func (projectQueryStub) List(context.Context, string, string, int32, int32) (models.ProjectListResponse, error) {
	return models.ProjectListResponse{Projects: []models.Project{}}, nil
}

func (projectQueryStub) Detail(_ context.Context, projectID int64) (models.Project, error) {
	return models.Project{ProjectID: projectID}, nil
}

// The project endpoints are admin-only, so authorization is asserted through
// the real router rather than the handler on its own.
func TestProjectRoutesRequireAdmin(t *testing.T) {
	t.Parallel()

	handler := NewHandler(Dependencies{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		// Both handlers claim /projects, so they are mounted together here to
		// prove the query and mutation routes coexist on one router.
		ProjectHandler:      handlers.NewProjectHandler(projectQueryStub{}),
		ProjectAdminHandler: handlers.NewProjectAdminHandler(projectMutationStub{}),
		TokenVerifier:       productTokenVerifier{},
	})
	const body = `{"name":"Engineering Fair","start_date":"2026-09-05","end_date":"2026-09-07"}`
	tests := []struct {
		method, path, body string
		adminStatus        int
	}{
		{http.MethodGet, "/projects", "", http.StatusOK},
		{http.MethodGet, "/projects/7", "", http.StatusOK},
		{http.MethodPost, "/projects", body, http.StatusCreated},
		{http.MethodPut, "/projects/7", body, http.StatusOK},
		{http.MethodDelete, "/projects/7", "", http.StatusNoContent},
	}

	for _, test := range tests {
		for _, identity := range []struct {
			name, authorization string
			want                int
		}{
			{"anonymous", "", http.StatusUnauthorized},
			{"user", "Bearer user", http.StatusForbidden},
			{"admin", "Bearer admin", test.adminStatus},
		} {
			t.Run(identity.name+"_"+test.method+"_"+test.path, func(t *testing.T) {
				request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
				request.Header.Set("Authorization", identity.authorization)
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != identity.want {
					t.Fatalf("status=%d want=%d body=%s", response.Code, identity.want, response.Body.String())
				}
			})
		}
	}
}
