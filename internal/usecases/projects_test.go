package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

type projectReaderStub struct {
	today  models.Date
	filter models.ProjectFilter
	offset int32
	limit  int32
}

func (stub *projectReaderStub) List(_ context.Context, today models.Date, filter models.ProjectFilter, offset, limit int32) ([]models.Project, int64, error) {
	stub.today, stub.filter, stub.offset, stub.limit = today, filter, offset, limit
	return []models.Project{}, 12, nil
}

func (stub *projectReaderStub) Detail(_ context.Context, today models.Date, _ int64) (models.Project, error) {
	stub.today = today
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
			response, err := NewProjectService(stub).List(context.Background(), "", "", test.page, test.pageSize)
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
	if _, err := NewProjectService(stub).List(context.Background(), "  fair  ", "ACTIVE", 1, 10); err != nil {
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
	if _, err := NewProjectService(stub).List(context.Background(), "   ", "", 1, 10); err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if stub.filter.Name != nil || stub.filter.Status != nil {
		t.Errorf("filter = %+v, want both fields nil", stub.filter)
	}
}

func TestProjectServiceListRejectsUnknownStatus(t *testing.T) {
	if _, err := NewProjectService(&projectReaderStub{}).List(context.Background(), "", "RUNNING", 1, 10); err == nil {
		t.Fatal("expected an error for an unknown status filter")
	}
}

func TestProjectServiceDerivesTodayInBangkok(t *testing.T) {
	stub := &projectReaderStub{}
	service := NewProjectService(stub)
	// 23:30 UTC is already the next calendar day in Bangkok.
	service.now = func() time.Time { return time.Date(2026, time.August, 27, 23, 30, 0, 0, time.UTC) }

	if _, err := service.List(context.Background(), "", "", 1, 10); err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if got := stub.today.String(); got != "2026-08-28" {
		t.Errorf("today = %q, want %q", got, "2026-08-28")
	}
}

func TestProjectServiceDetailRejectsNonPositiveID(t *testing.T) {
	if _, err := NewProjectService(&projectReaderStub{}).Detail(context.Background(), 0); err == nil {
		t.Fatal("expected an error for a non-positive project ID")
	}
}
