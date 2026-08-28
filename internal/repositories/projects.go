package repositories

import (
	"context"
	"errors"
	"fmt"
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
// SQL and Go definitions of "today" can never disagree.
const projectStatusExpression = `CASE
    WHEN $1::date < p.start_date THEN 'NOT_STARTED'
    WHEN $1::date > p.end_date THEN 'COMPLETED'
    ELSE 'ACTIVE'
END`

// projectSelection joins the per-project order count as a single aggregate so
// that listing a page never issues one query per project.
const projectSelection = `SELECT p.project_id, p.name, p.description, p.start_date, p.end_date,
       p.created_at, p.updated_at, COALESCE(o.order_count, 0),
       ` + projectStatusExpression + `
FROM projects p
LEFT JOIN (
    SELECT project_id, COUNT(*) AS order_count
    FROM orders
    WHERE project_id IS NOT NULL
    GROUP BY project_id
) o ON o.project_id = p.project_id`

// projectFilterClause keeps both filters optional and independent so that name
// and status can be combined.
const projectFilterClause = ` WHERE ($2::text IS NULL OR p.name ILIKE '%' || $2 || '%')
  AND ($3::text IS NULL OR ` + projectStatusExpression + ` = $3)`

// List returns one filtered, ordered page of projects and the filtered total.
func (repository *ProjectRepository) List(ctx context.Context, today models.Date, filter models.ProjectFilter, offset, limit int32) ([]models.Project, int64, error) {
	name, status := filterArguments(filter)

	rows, err := repository.pool.Query(ctx,
		projectSelection+projectFilterClause+` ORDER BY p.created_at DESC, p.project_id DESC OFFSET $4 LIMIT $5`,
		today.Time, name, status, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]models.Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, 0, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate projects: %w", err)
	}

	var total int64
	if err := repository.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM projects p`+projectFilterClause, today.Time, name, status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}
	return projects, total, nil
}

// Detail returns a single project with its derived status and order count.
func (repository *ProjectRepository) Detail(ctx context.Context, today models.Date, projectID int64) (models.Project, error) {
	rows, err := repository.pool.Query(ctx, projectSelection+` WHERE p.project_id = $2`, today.Time, projectID)
	if err != nil {
		return models.Project{}, fmt.Errorf("get project: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return models.Project{}, fmt.Errorf("get project: %w", err)
		}
		return models.Project{}, ErrProjectNotFound
	}
	return scanProject(rows)
}

func filterArguments(filter models.ProjectFilter) (*string, *string) {
	var status *string
	if filter.Status != nil {
		value := string(*filter.Status)
		status = &value
	}
	return filter.Name, status
}

func scanProject(rows pgx.Rows) (models.Project, error) {
	var project models.Project
	var startDate, endDate time.Time
	var status string
	if err := rows.Scan(&project.ProjectID, &project.Name, &project.Description, &startDate, &endDate,
		&project.CreatedAt, &project.UpdatedAt, &project.OrderCount, &status); err != nil {
		return models.Project{}, fmt.Errorf("scan project: %w", err)
	}
	project.StartDate = models.NewDate(startDate)
	project.EndDate = models.NewDate(endDate)
	project.Status = models.ProjectStatus(status)
	return project, nil
}
