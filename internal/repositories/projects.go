package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrProjectNotFound reports a project ID with no matching row.
var ErrProjectNotFound = errors.New("project not found")

// ProjectRepository reads projects and their derived status and order count.
type ProjectRepository struct{ pool *pgxpool.Pool }

// NewProjectRepository constructs a project repository over the connection pool.
func NewProjectRepository(pool *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{pool: pool}
}

// projectStatusExpression derives the status from the supplied calendar date.
// The date is passed in rather than read from the database clock so that the
// SQL and Go definitions of "today" can never disagree;
// TestProjectRepositoryStatusMatchesTheDomainRule holds the two in agreement.
const projectStatusExpression = `CASE
    WHEN $1::date < p.start_date THEN 'NOT_STARTED'
    WHEN $1::date > p.end_date THEN 'COMPLETED'
    ELSE 'ACTIVE'
END`

// projectOrderCount counts orders once per project rather than once per
// listed row, so that listing a page never issues one query per project.
const projectOrderCount = `LEFT JOIN (
    SELECT project_id, COUNT(*) AS order_count
    FROM orders
    WHERE project_id IS NOT NULL
    GROUP BY project_id
) o ON o.project_id = p.project_id`

// projectColumns is the shared projection for a single project row.
const projectColumns = `p.project_id, p.name, p.description, p.start_date, p.end_date,
       p.created_at, p.updated_at, COALESCE(o.order_count, 0),
       ` + projectStatusExpression

// projectFilterClause keeps both filters optional and independent so that name
// and status can be combined. The name is matched literally: LIKE wildcards in
// the filter are escaped by escapeLikePattern before the query runs.
const projectFilterClause = ` WHERE ($2::text IS NULL OR p.name ILIKE '%' || $2 || '%' ESCAPE '\')
  AND ($3::text IS NULL OR ` + projectStatusExpression + ` = $3)`

// listProjectsQuery reads the page and its filtered total together, so that
// both describe the same snapshot of the table.
const listProjectsQuery = `SELECT ` + projectColumns + `, COUNT(*) OVER () AS total
FROM projects p
` + projectOrderCount + projectFilterClause + `
ORDER BY p.created_at DESC, p.project_id DESC
OFFSET $4 LIMIT $5`

const detailProjectQuery = `SELECT ` + projectColumns + `
FROM projects p
` + projectOrderCount + `
WHERE p.project_id = $2`

// List returns one filtered, ordered page of projects and the filtered total.
func (repository *ProjectRepository) List(ctx context.Context, today models.Date, filter models.ProjectFilter, offset, limit int32) ([]models.Project, int64, error) {
	name, status := filterArguments(filter)

	rows, err := repository.pool.Query(ctx, listProjectsQuery, today.Time, name, status, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]models.Project, 0)
	var total int64
	for rows.Next() {
		project, err := scanProject(rows, &total)
		if err != nil {
			return nil, 0, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate projects: %w", err)
	}

	// A page past the end of the result set returns no rows, and therefore no
	// window count, so the total has to be read separately in that case.
	if len(projects) == 0 {
		if total, err = repository.count(ctx, today, name, status); err != nil {
			return nil, 0, err
		}
	}
	return projects, total, nil
}

// Detail returns a single project with its derived status and order count.
func (repository *ProjectRepository) Detail(ctx context.Context, today models.Date, projectID int64) (models.Project, error) {
	rows, err := repository.pool.Query(ctx, detailProjectQuery, today.Time, projectID)
	if err != nil {
		return models.Project{}, fmt.Errorf("get project: %w", err)
	}
	project, err := pgx.CollectExactlyOneRow(rows, func(row pgx.CollectableRow) (models.Project, error) {
		return scanProject(row)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Project{}, ErrProjectNotFound
	}
	if err != nil {
		return models.Project{}, fmt.Errorf("get project: %w", err)
	}
	return project, nil
}

func (repository *ProjectRepository) count(ctx context.Context, today models.Date, name, status *string) (int64, error) {
	var total int64
	if err := repository.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM projects p`+projectFilterClause, today.Time, name, status).Scan(&total); err != nil {
		return 0, fmt.Errorf("count projects: %w", err)
	}
	return total, nil
}

// escapeLikePattern neutralises the LIKE wildcards, so that a filter such as
// "50%" matches a literal per cent sign rather than every project.
func escapeLikePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func filterArguments(filter models.ProjectFilter) (*string, *string) {
	var name *string
	if filter.Name != nil {
		escaped := escapeLikePattern(*filter.Name)
		name = &escaped
	}
	var status *string
	if filter.Status != nil {
		value := string(*filter.Status)
		status = &value
	}
	return name, status
}

// rowScanner is the scan surface shared by pgx.Rows and pgx.CollectableRow.
type rowScanner interface{ Scan(dest ...any) error }

// scanProject reads the shared project projection, appending any extra
// destinations the caller selected after those columns.
func scanProject(row rowScanner, extra ...any) (models.Project, error) {
	var project models.Project
	var startDate, endDate time.Time
	var status string
	destinations := append([]any{
		&project.ProjectID, &project.Name, &project.Description, &startDate, &endDate,
		&project.CreatedAt, &project.UpdatedAt, &project.OrderCount, &status,
	}, extra...)
	if err := row.Scan(destinations...); err != nil {
		return models.Project{}, fmt.Errorf("scan project: %w", err)
	}
	project.StartDate = models.NewDate(startDate)
	project.EndDate = models.NewDate(endDate)
	project.Status = models.ProjectStatus(status)
	return project, nil
}
