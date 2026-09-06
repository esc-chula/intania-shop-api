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
var (
	ErrProjectNotFound       = errors.New("project not found")
	ErrProjectCompleted      = errors.New("project is completed")
	ErrProjectProductInvalid = errors.New("invalid project product assignment")
)

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

func (repository *ProjectRepository) ReplaceProducts(ctx context.Context, today models.Date, projectID int64, items []models.ProjectProductAssignmentInput) ([]models.ProjectProductAssignment, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin project product replacement: %w", err)
	}
	defer rollback(ctx, tx)
	var endDate time.Time
	if err = tx.QueryRow(ctx, `SELECT end_date FROM projects WHERE project_id=$1 FOR UPDATE`, projectID).Scan(&endDate); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("lock project: %w", err)
	}
	if today.After(endDate) {
		return nil, ErrProjectCompleted
	}
	for _, item := range items {
		var valid bool
		if item.VariantID == nil {
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products p WHERE p.id=$1 AND NOT EXISTS (SELECT 1 FROM variants v WHERE v.product_id=p.id))`, item.ProductID).Scan(&valid)
		} else {
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM variants WHERE variant_id=$1 AND product_id=$2)`, *item.VariantID, item.ProductID).Scan(&valid)
		}
		if err != nil {
			return nil, fmt.Errorf("validate project product: %w", err)
		}
		if !valid {
			return nil, ErrProjectProductInvalid
		}
	}
	// TODO(BE-006): after project promotions exist, reject this replacement
	// with a conflict when it removes or reprices an item used by a promotion.
	if _, err = tx.Exec(ctx, `DELETE FROM project_products WHERE project_id=$1`, projectID); err != nil {
		return nil, fmt.Errorf("clear project products: %w", err)
	}
	for _, item := range items {
		if _, err = tx.Exec(ctx, `INSERT INTO project_products(project_id, product_id, variant_id, project_price) VALUES ($1,$2,$3,$4::numeric)`, projectID, item.ProductID, item.VariantID, item.ProjectPrice); err != nil {
			return nil, fmt.Errorf("insert project product: %w", err)
		}
	}
	rows, err := tx.Query(ctx, `SELECT pp.product_id, pp.variant_id, p.name, p.category, p.images[1], v.size, v.color, COALESCE(v.stock_quantity, p.stock_quantity, 0), pp.project_price::text FROM project_products pp JOIN products p ON p.id=pp.product_id LEFT JOIN variants v ON v.variant_id=pp.variant_id WHERE pp.project_id=$1 ORDER BY pp.product_id, pp.variant_id NULLS FIRST`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project products: %w", err)
	}
	defer rows.Close()
	out := make([]models.ProjectProductAssignment, 0)
	for rows.Next() {
		var item models.ProjectProductAssignment
		if err := rows.Scan(&item.ProductID, &item.VariantID, &item.ProductName, &item.Category, &item.ImageURL, &item.Size, &item.Color, &item.StockQuantity, &item.ProjectPrice); err != nil {
			return nil, fmt.Errorf("scan project product: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project products: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit project product replacement: %w", err)
	}
	return out, nil
}

const projectProductFilterClause = `WHERE ($1::text = '' OR p.name ILIKE '%' || $1 || '%' ESCAPE '\\')
  AND ($2::text = '' OR lower(p.category) = lower($2))`

const listProductCandidatesQuery = `WITH page AS (
    SELECT p.id, p.name, p.category, p.images[1] AS image_url, p.price, p.stock_quantity,
           COUNT(*) OVER () AS total
    FROM products p
    ` + projectProductFilterClause + `
    ORDER BY p.id DESC
    OFFSET $4 LIMIT $5
)
SELECT page.id, page.name, page.category, page.image_url, page.total,
       v.variant_id, v.size, v.color,
       COALESCE(CASE WHEN v.variant_id IS NULL THEN page.stock_quantity ELSE v.stock_quantity END, 0),
       COALESCE(v.price, page.price)::text,
       pp.project_id IS NOT NULL,
       pp.project_price::text
FROM page
LEFT JOIN variants v ON v.product_id = page.id
LEFT JOIN project_products pp ON pp.project_id = $3
       AND pp.product_id = page.id
       AND pp.variant_id IS NOT DISTINCT FROM v.variant_id
ORDER BY page.id DESC, v.variant_id NULLS FIRST`

const countProductCandidatesQuery = `SELECT COUNT(*) FROM products p ` + projectProductFilterClause

const listProjectProductsQuery = `SELECT pp.product_id, pp.variant_id, p.name, p.category, p.images[1],
       v.size, v.color,
       COALESCE(CASE WHEN pp.variant_id IS NULL THEN p.stock_quantity ELSE v.stock_quantity END, 0),
       pp.project_price::text
FROM project_products pp
JOIN products p ON p.id = pp.product_id
LEFT JOIN variants v ON v.variant_id = pp.variant_id
WHERE pp.project_id = $1
ORDER BY pp.product_id, pp.variant_id NULLS FIRST`

func (repository *ProjectRepository) ListProductCandidates(ctx context.Context, projectID int64, filter models.ProjectProductFilter, offset, limit int32) ([]models.ProjectProductCandidate, int64, error) {
	if err := repository.requireProject(ctx, projectID); err != nil {
		return nil, 0, err
	}
	name := escapeLikePattern(filter.Name)

	rows, err := repository.pool.Query(ctx, listProductCandidatesQuery, name, filter.Category, projectID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list product candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]models.ProjectProductCandidate, 0)
	var total int64
	for rows.Next() {
		var candidate models.ProjectProductCandidate
		var item models.ProjectSellableItem
		if err := rows.Scan(&candidate.ProductID, &candidate.Name, &candidate.Category, &candidate.ImageURL, &total,
			&item.VariantID, &item.Size, &item.Color, &item.StockQuantity, &item.DefaultPrice,
			&item.Selected, &item.ProjectPrice); err != nil {
			return nil, 0, fmt.Errorf("scan product candidate: %w", err)
		}
		item.ProductID = candidate.ProductID
		if len(candidates) == 0 || candidates[len(candidates)-1].ProductID != candidate.ProductID {
			candidate.SellableItems = make([]models.ProjectSellableItem, 0, 1)
			candidates = append(candidates, candidate)
		}
		last := &candidates[len(candidates)-1]
		last.SellableItems = append(last.SellableItems, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate product candidates: %w", err)
	}
	if len(candidates) == 0 {
		if err := repository.pool.QueryRow(ctx, countProductCandidatesQuery, name, filter.Category).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count product candidates: %w", err)
		}
	}
	return candidates, total, nil
}

func (repository *ProjectRepository) ListProjectProducts(ctx context.Context, projectID int64) ([]models.ProjectProductAssignment, error) {
	if err := repository.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	rows, err := repository.pool.Query(ctx, listProjectProductsQuery, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project products: %w", err)
	}
	defer rows.Close()

	items := make([]models.ProjectProductAssignment, 0)
	for rows.Next() {
		var item models.ProjectProductAssignment
		if err := rows.Scan(&item.ProductID, &item.VariantID, &item.ProductName, &item.Category, &item.ImageURL,
			&item.Size, &item.Color, &item.StockQuantity, &item.ProjectPrice); err != nil {
			return nil, fmt.Errorf("scan project product: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project products: %w", err)
	}
	return items, nil
}

func (repository *ProjectRepository) requireProject(ctx context.Context, projectID int64) error {
	var exists int
	err := repository.pool.QueryRow(ctx, `SELECT 1 FROM projects WHERE project_id = $1`, projectID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProjectNotFound
	}
	if err != nil {
		return fmt.Errorf("check project: %w", err)
	}
	return nil
}
