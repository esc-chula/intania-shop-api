//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/migrations"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/jackc/pgx/v5/pgxpool"
)

// projectToday pins the calendar date so that derived statuses stay stable over time.
const projectToday = "2026-08-28"

type seededProject struct {
	name       string
	startDate  string
	endDate    string
	orderCount int
}

// seedProjects inserts projects in a known creation order and returns their IDs
// keyed by project name. Later entries are created later, so they list first.
func seedProjects(t *testing.T, pool *pgxpool.Pool) map[string]int64 {
	t.Helper()
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `TRUNCATE orders, projects RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate project tables: %v", err)
	}

	projects := []seededProject{
		{name: "Engineering Fair", startDate: "2026-09-05", endDate: "2026-09-07"},
		{name: "freshy night", startDate: "2026-08-01", endDate: "2026-08-03", orderCount: 2},
		{name: "Engineering Open House", startDate: "2026-08-28", endDate: "2026-08-28", orderCount: 3},
		{name: "Sports Day", startDate: "2026-08-27", endDate: "2026-08-30"},
		{name: "Alumni Homecoming", startDate: "2026-12-01", endDate: "2026-12-02"},
	}

	ids := make(map[string]int64, len(projects))
	for index, project := range projects {
		var projectID int64
		// created_at is set explicitly so that the listing order is deterministic.
		if err := pool.QueryRow(ctx,
			`INSERT INTO projects(name, start_date, end_date, created_at)
			 VALUES ($1, $2::date, $3::date, TIMESTAMP '2026-08-01 00:00:00' + make_interval(hours => $4))
			 RETURNING project_id`,
			project.name, project.startDate, project.endDate, index).Scan(&projectID); err != nil {
			t.Fatalf("insert project %q: %v", project.name, err)
		}
		ids[project.name] = projectID
		for range project.orderCount {
			if _, err := pool.Exec(ctx, `INSERT INTO orders(project_id) VALUES ($1)`, projectID); err != nil {
				t.Fatalf("insert order for %q: %v", project.name, err)
			}
		}
	}
	// An order with no project must never be counted against a project.
	if _, err := pool.Exec(ctx, `INSERT INTO orders(project_id) VALUES (NULL)`); err != nil {
		t.Fatalf("insert unassigned order: %v", err)
	}
	return ids
}

func projectTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	if err := migrations.Apply(ctx, dsn, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func projectDate(t *testing.T, value string) models.Date {
	t.Helper()

	date, err := models.ParseDate(value)
	if err != nil {
		t.Fatalf("parse date %q: %v", value, err)
	}
	return date
}

func projectNames(projects []models.Project) []string {
	names := make([]string, 0, len(projects))
	for _, project := range projects {
		names = append(names, project.Name)
	}
	return names
}

func equalNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func TestProjectRepositoryDetailDerivesStatusAndOrderCount(t *testing.T) {
	pool := projectTestPool(t)
	ids := seedProjects(t, pool)
	repository := repositories.NewProjectRepository(pool)

	tests := []struct {
		projectName string
		wantStatus  models.ProjectStatus
		wantOrders  int64
	}{
		{projectName: "Engineering Fair", wantStatus: models.ProjectStatusNotStarted, wantOrders: 0},
		{projectName: "freshy night", wantStatus: models.ProjectStatusCompleted, wantOrders: 2},
		{projectName: "Engineering Open House", wantStatus: models.ProjectStatusActive, wantOrders: 3},
		{projectName: "Sports Day", wantStatus: models.ProjectStatusActive, wantOrders: 0},
	}

	for _, test := range tests {
		t.Run(test.projectName, func(t *testing.T) {
			project, err := repository.Detail(context.Background(), projectDate(t, projectToday), ids[test.projectName])
			if err != nil {
				t.Fatalf("get project detail: %v", err)
			}
			if project.Status != test.wantStatus {
				t.Errorf("status=%q want %q", project.Status, test.wantStatus)
			}
			if project.OrderCount != test.wantOrders {
				t.Errorf("order_count=%d want %d", project.OrderCount, test.wantOrders)
			}
			if project.Name != test.projectName {
				t.Errorf("name=%q want %q", project.Name, test.projectName)
			}
		})
	}
}

func TestProjectRepositoryDetailReportsMissingProject(t *testing.T) {
	pool := projectTestPool(t)
	seedProjects(t, pool)

	_, err := repositories.NewProjectRepository(pool).Detail(context.Background(), projectDate(t, projectToday), 999999)
	if !errors.Is(err, repositories.ErrProjectNotFound) {
		t.Fatalf("err=%v want %v", err, repositories.ErrProjectNotFound)
	}
}

func TestProjectRepositoryListOrdersNewestFirstAndIsStable(t *testing.T) {
	pool := projectTestPool(t)
	seedProjects(t, pool)
	repository := repositories.NewProjectRepository(pool)

	want := []string{"Alumni Homecoming", "Sports Day", "Engineering Open House", "freshy night", "Engineering Fair"}
	for attempt := range 3 {
		projects, total, err := repository.List(context.Background(), projectDate(t, projectToday), models.ProjectFilter{}, 0, 10)
		if err != nil {
			t.Fatalf("list projects: %v", err)
		}
		if total != 5 {
			t.Fatalf("total=%d want 5", total)
		}
		if got := projectNames(projects); !equalNames(got, want) {
			t.Fatalf("attempt %d order=%v want %v", attempt, got, want)
		}
	}
}

func TestProjectRepositoryListFiltersByNameCaseInsensitively(t *testing.T) {
	pool := projectTestPool(t)
	seedProjects(t, pool)

	name := "ENGIN"
	projects, total, err := repositories.NewProjectRepository(pool).List(
		context.Background(), projectDate(t, projectToday), models.ProjectFilter{Name: &name}, 0, 10)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	want := []string{"Engineering Open House", "Engineering Fair"}
	if total != 2 || !equalNames(projectNames(projects), want) {
		t.Fatalf("total=%d names=%v, want 2 and %v", total, projectNames(projects), want)
	}
}

func TestProjectRepositoryListFiltersByStatus(t *testing.T) {
	pool := projectTestPool(t)
	seedProjects(t, pool)
	repository := repositories.NewProjectRepository(pool)

	tests := []struct {
		status models.ProjectStatus
		want   []string
	}{
		{status: models.ProjectStatusNotStarted, want: []string{"Alumni Homecoming", "Engineering Fair"}},
		{status: models.ProjectStatusActive, want: []string{"Sports Day", "Engineering Open House"}},
		{status: models.ProjectStatusCompleted, want: []string{"freshy night"}},
	}

	for _, test := range tests {
		t.Run(string(test.status), func(t *testing.T) {
			status := test.status
			projects, total, err := repository.List(
				context.Background(), projectDate(t, projectToday), models.ProjectFilter{Status: &status}, 0, 10)
			if err != nil {
				t.Fatalf("list projects: %v", err)
			}
			if total != int64(len(test.want)) || !equalNames(projectNames(projects), test.want) {
				t.Fatalf("total=%d names=%v, want %d and %v", total, projectNames(projects), len(test.want), test.want)
			}
		})
	}
}

func TestProjectRepositoryListCombinesNameAndStatusFilters(t *testing.T) {
	pool := projectTestPool(t)
	seedProjects(t, pool)

	name := "engineering"
	status := models.ProjectStatusActive
	projects, total, err := repositories.NewProjectRepository(pool).List(
		context.Background(), projectDate(t, projectToday), models.ProjectFilter{Name: &name, Status: &status}, 0, 10)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	want := []string{"Engineering Open House"}
	if total != 1 || !equalNames(projectNames(projects), want) {
		t.Fatalf("total=%d names=%v, want 1 and %v", total, projectNames(projects), want)
	}
}

func TestProjectRepositoryListAppliesFiltersBeforePagination(t *testing.T) {
	pool := projectTestPool(t)
	seedProjects(t, pool)
	repository := repositories.NewProjectRepository(pool)

	name := "e"
	filter := models.ProjectFilter{Name: &name}

	firstPage, total, err := repository.List(context.Background(), projectDate(t, projectToday), filter, 0, 2)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	secondPage, secondTotal, err := repository.List(context.Background(), projectDate(t, projectToday), filter, 2, 2)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}

	// Four of the five seeded names contain "e"; "Sports Day" does not.
	if total != 4 || secondTotal != 4 {
		t.Fatalf("totals=%d and %d, want 4 on both pages", total, secondTotal)
	}
	if len(firstPage) != 2 || len(secondPage) != 2 {
		t.Fatalf("page sizes=%d and %d, want 2 on both pages", len(firstPage), len(secondPage))
	}
	for _, first := range firstPage {
		for _, second := range secondPage {
			if first.ProjectID == second.ProjectID {
				t.Fatalf("project %d appears on both pages", first.ProjectID)
			}
		}
	}

	// The total must report the filtered count rather than the table count.
	narrow := "Sports"
	_, narrowTotal, err := repository.List(context.Background(), projectDate(t, projectToday), models.ProjectFilter{Name: &narrow}, 0, 2)
	if err != nil {
		t.Fatalf("list narrowed page: %v", err)
	}
	if narrowTotal != 1 {
		t.Fatalf("narrowed total=%d want 1", narrowTotal)
	}
}

// The listing derives status in SQL, while models.ProjectStatusFor states the
// same rule in Go. Nothing in the type system holds the two together, so this
// walks a date range across every seeded project and fails the moment the
// database disagrees with the domain rule.
func TestProjectRepositoryStatusMatchesTheDomainRule(t *testing.T) {
	pool := projectTestPool(t)
	seedProjects(t, pool)
	repository := repositories.NewProjectRepository(pool)

	for dayOffset := range 200 {
		today := models.NewDate(time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, dayOffset))
		projects, _, err := repository.List(context.Background(), today, models.ProjectFilter{}, 0, 100)
		if err != nil {
			t.Fatalf("list projects on %s: %v", today, err)
		}
		for _, project := range projects {
			want := models.ProjectStatusFor(project.StartDate, project.EndDate, today)
			if project.Status != want {
				t.Fatalf("on %s, project %q: SQL says %q, models.ProjectStatusFor says %q",
					today, project.Name, project.Status, want)
			}
		}
	}
}

func TestProjectRepositoryListMatchesNameWildcardsLiterally(t *testing.T) {
	pool := projectTestPool(t)
	seedProjects(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO projects(name, start_date, end_date) VALUES ($1, DATE '2026-08-01', DATE '2026-08-02')`,
		"50% off booth"); err != nil {
		t.Fatalf("insert project with a wildcard in its name: %v", err)
	}

	repository := repositories.NewProjectRepository(pool)
	for _, filter := range []string{"50%", "_"} {
		t.Run(filter, func(t *testing.T) {
			name := filter
			projects, total, err := repository.List(ctx, projectDate(t, projectToday), models.ProjectFilter{Name: &name}, 0, 10)
			if err != nil {
				t.Fatalf("list projects: %v", err)
			}
			// "50%" matches only the project containing it; "_" matches nothing,
			// rather than every project as an unescaped wildcard would.
			want := 0
			if filter == "50%" {
				want = 1
			}
			if int(total) != want || len(projects) != want {
				t.Fatalf("filter %q matched total=%d rows=%d, want %d", filter, total, len(projects), want)
			}
		})
	}
}

func TestProjectRepositoryListReportsTheTotalPastTheLastPage(t *testing.T) {
	pool := projectTestPool(t)
	seedProjects(t, pool)

	projects, total, err := repositories.NewProjectRepository(pool).List(
		context.Background(), projectDate(t, projectToday), models.ProjectFilter{}, 100, 10)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 0 || total != 5 {
		t.Fatalf("rows=%d total=%d, want 0 and 5", len(projects), total)
	}
}
