package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

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
