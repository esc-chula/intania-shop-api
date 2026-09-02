package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrProjectNotFound reports a project ID with no matching row.
var ErrProjectNotFound = errors.New("project not found")

// ProjectRepository reads and writes projects and their derived status and order count.
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

const projectFilterClause = ` WHERE ($2::text = '' OR p.name ILIKE '%' || $2 || '%' ESCAPE '\')
  AND ($3::text = '' OR ` + projectStatusExpression + ` = $3)`

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
	name := escapeLikePattern(filter.Name)
	status := string(filter.Status)

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
	project, err := scanProject(repository.pool.QueryRow(ctx, detailProjectQuery, today.Time, projectID))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Project{}, ErrProjectNotFound
	}
	if err != nil {
		return models.Project{}, fmt.Errorf("get project: %w", err)
	}
	return project, nil
}

func (repository *ProjectRepository) count(ctx context.Context, today models.Date, name, status string) (int64, error) {
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

// ErrProjectHasOrders reports a project that cannot be deleted because orders
// still reference it.
var ErrProjectHasOrders = errors.New("project has orders")

// foreignKeyViolation is the SQLSTATE Postgres raises when ON DELETE RESTRICT
// stops a delete.
const foreignKeyViolation = "23503"

const createProjectQuery = `INSERT INTO projects (name, description, start_date, end_date)
VALUES ($1, $2, $3::date, $4::date)
RETURNING project_id`

// updateProjectQuery replaces every editable column, so an update is a complete
// replacement. updated_at is set here because the table has no trigger for it.
const updateProjectQuery = `UPDATE projects
SET name = $2, description = $3, start_date = $4::date, end_date = $5::date,
    updated_at = CURRENT_TIMESTAMP
WHERE project_id = $1`

// Create inserts a project and reads it back through the shared projection, so
// that a created project is described exactly like a listed one. The input has
// already been validated by the use case.
func (repository *ProjectRepository) Create(ctx context.Context, today models.Date, input models.ProjectInput) (models.Project, error) {
	var projectID int64
	if err := repository.pool.QueryRow(ctx, createProjectQuery,
		*input.Name, input.Description, input.StartDate.Time, input.EndDate.Time,
	).Scan(&projectID); err != nil {
		return models.Project{}, fmt.Errorf("create project: %w", err)
	}
	return repository.Detail(ctx, today, projectID)
}

// Update replaces the editable columns of an existing project and reads it back.
func (repository *ProjectRepository) Update(ctx context.Context, today models.Date, projectID int64, input models.ProjectInput) (models.Project, error) {
	tag, err := repository.pool.Exec(ctx, updateProjectQuery,
		projectID, *input.Name, input.Description, input.StartDate.Time, input.EndDate.Time)
	if err != nil {
		return models.Project{}, fmt.Errorf("update project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.Project{}, ErrProjectNotFound
	}
	return repository.Detail(ctx, today, projectID)
}

// Delete permanently removes a project. Orders reference projects with
// ON DELETE RESTRICT, so the database is what enforces that a project with
// sales is never deleted.
func (repository *ProjectRepository) Delete(ctx context.Context, projectID int64) error {
	tag, err := repository.pool.Exec(ctx, `DELETE FROM projects WHERE project_id = $1`, projectID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolation {
			return ErrProjectHasOrders
		}
		return fmt.Errorf("delete project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrProjectNotFound
	}
	return nil
}
