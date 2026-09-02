package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
)

type projectReaderStub struct {
	today     models.Date
	filter    models.ProjectFilter
	offset    int32
	limit     int32
	detailErr error
}

func (stub *projectReaderStub) List(_ context.Context, today models.Date, filter models.ProjectFilter, offset, limit int32) ([]models.Project, int64, error) {
	stub.today, stub.filter, stub.offset, stub.limit = today, filter, offset, limit
	return []models.Project{}, 12, nil
}

func (stub *projectReaderStub) Detail(_ context.Context, today models.Date, _ int64) (models.Project, error) {
	stub.today = today
	if stub.detailErr != nil {
		return models.Project{}, stub.detailErr
	}
	return models.Project{ProjectID: 1}, nil
}

func TestProjectServiceListNormalizesPagination(t *testing.T) {
	tests := []struct {
		name                             string
		page, pageSize                   int32
		wantOffset, wantLimit, wantPages int32
	}{
		{name: "defaults applied to zero values", page: 0, pageSize: 0, wantOffset: 0, wantLimit: 10, wantPages: 2},
		{name: "second page offsets by page size", page: 2, pageSize: 5, wantOffset: 5, wantLimit: 5, wantPages: 3},
		{name: "page size capped at 100", page: 1, pageSize: 500, wantOffset: 0, wantLimit: 100, wantPages: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stub := &projectReaderStub{}
			response, err := NewProjectService(stub, nil).List(context.Background(), "", "", test.page, test.pageSize)
			if err != nil {
				t.Fatalf("list projects: %v", err)
			}
			if stub.offset != test.wantOffset || stub.limit != test.wantLimit {
				t.Errorf("offset=%d limit=%d, want %d and %d", stub.offset, stub.limit, test.wantOffset, test.wantLimit)
			}
			if response.TotalPages != test.wantPages || response.Total != 12 {
				t.Errorf("total=%d total_pages=%d, want 12 and %d", response.Total, response.TotalPages, test.wantPages)
			}
		})
	}
}

func TestProjectServiceListBuildsFilters(t *testing.T) {
	stub := &projectReaderStub{}
	if _, err := NewProjectService(stub, nil).List(context.Background(), "  fair  ", "ACTIVE", 1, 10); err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if stub.filter.Name == nil || *stub.filter.Name != "fair" {
		t.Errorf("name filter = %v, want %q", stub.filter.Name, "fair")
	}
	if stub.filter.Status == nil || *stub.filter.Status != models.ProjectStatusActive {
		t.Errorf("status filter = %v, want %q", stub.filter.Status, models.ProjectStatusActive)
	}
}

func TestProjectServiceListOmitsBlankFilters(t *testing.T) {
	stub := &projectReaderStub{}
	if _, err := NewProjectService(stub, nil).List(context.Background(), "   ", "", 1, 10); err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if stub.filter.Name != nil || stub.filter.Status != nil {
		t.Errorf("filter = %+v, want both fields nil", stub.filter)
	}
}

func TestProjectServiceDerivesTodayInBangkok(t *testing.T) {
	stub := &projectReaderStub{}
	service := NewProjectService(stub, nil)
	// 23:30 UTC is already the next calendar day in Bangkok.
	service.now = func() time.Time { return time.Date(2026, time.August, 27, 23, 30, 0, 0, time.UTC) }

	if _, err := service.List(context.Background(), "", "", 1, 10); err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if got := stub.today.String(); got != "2026-08-28" {
		t.Errorf("today = %q, want %q", got, "2026-08-28")
	}
}

// The handler distinguishes 404 from 500 with errors.Is, so the repository
// sentinel has to stay matchable through the use case wrapping.
func TestProjectServiceDetailPreservesTheNotFoundSentinel(t *testing.T) {
	stub := &projectReaderStub{detailErr: fmt.Errorf("get project: %w", repositories.ErrProjectNotFound)}

	_, err := NewProjectService(stub, nil).Detail(context.Background(), 7)
	if !errors.Is(err, repositories.ErrProjectNotFound) {
		t.Fatalf("err=%v, want it to match %v", err, repositories.ErrProjectNotFound)
	}
}

func TestProjectServiceReportsTypedValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		call func(*ProjectService) error
		want error
	}{
		{
			name: "unknown status",
			call: func(service *ProjectService) error {
				_, err := service.List(context.Background(), "", "RUNNING", 1, 10)
				return err
			},
			want: ErrInvalidProjectStatus,
		},
		{
			name: "name filter longer than a project name",
			call: func(service *ProjectService) error {
				_, err := service.List(context.Background(), strings.Repeat("a", models.ProjectNameMaxLength+1), "", 1, 10)
				return err
			},
			want: ErrProjectNameTooLong,
		},
		{
			name: "non-positive ID",
			call: func(service *ProjectService) error {
				_, err := service.Detail(context.Background(), 0)
				return err
			},
			want: ErrInvalidProjectID,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(NewProjectService(&projectReaderStub{}, nil)); !errors.Is(err, test.want) {
				t.Fatalf("err=%v, want it to match %v", err, test.want)
			}
		})
	}
}
