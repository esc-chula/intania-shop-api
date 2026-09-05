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

var (
	// ErrPromotionNotFound reports a promotion that does not belong to the
	// requested project or does not exist.
	ErrPromotionNotFound = errors.New("promotion not found")
	// ErrProductNotSellable reports an item that is not assigned to the project.
	ErrProductNotSellable = errors.New("product is not sellable in project")
	// ErrProjectCompleted reports a mutation attempted after the project ended.
	ErrProjectCompleted = errors.New("project is completed")
	// ErrPromotionPriceExceedsBundle reports a mutation whose fixed price is
	// greater than the current project-product bundle price.
	ErrPromotionPriceExceedsBundle = errors.New("promotion price exceeds bundle price")
	// ErrPromotionPricingCorrupt reports persisted data that cannot produce a
	// valid promotion price response.
	ErrPromotionPricingCorrupt = errors.New("promotion pricing data is corrupt")
)

// ProjectProductAssignment is the internal assignment identity resolved from
// a public product/variant reference. project_product_id is never serialized
// in the Promotion API.
type ProjectProductAssignment struct {
	ProjectProductID int64
	ProjectID        int64
	ProductID        int64
	VariantID        *int64
	ProjectPrice     models.THBAmount
}

// PromotionReader is the read-only persistence surface for promotion
// consumers such as POS pricing.
type PromotionReader interface {
	List(context.Context, int64) ([]models.ProjectPromotion, error)
	Detail(context.Context, int64, int64) (models.ProjectPromotion, error)
}

// PromotionWriter is the mutation persistence surface for the admin
// promotion use case. Create and Update perform their resolution and writes
// in one transaction.
type PromotionWriter interface {
	Create(context.Context, models.Date, int64, models.ProjectPromotionMutation) (models.ProjectPromotion, error)
	Update(context.Context, models.Date, int64, int64, models.ProjectPromotionMutation) (models.ProjectPromotion, error)
	Delete(context.Context, models.Date, int64, int64) error
}

// PromotionStore combines read and write capabilities for consumers such as
// the admin promotion service that needs the complete CRUD surface.
type PromotionStore interface {
	PromotionReader
	PromotionWriter
}

// ProjectProductResolver resolves public promotion item references to the
// internal project-product assignments. The PromotionRepository uses the
// transaction-aware equivalent during mutations so validation and writes are
// protected by the same row locks.
type ProjectProductResolver interface {
	ResolveProjectProducts(context.Context, int64, []models.ProjectPromotionItemInput) ([]ProjectProductAssignment, error)
}

// PromotionRepository persists project-scoped promotions and their items.
type PromotionRepository struct{ pool *pgxpool.Pool }

var (
	_ PromotionReader = (*PromotionRepository)(nil)
	_ PromotionWriter = (*PromotionRepository)(nil)
	_ PromotionStore  = (*PromotionRepository)(nil)
)

// NewPromotionRepository constructs a Promotion repository over the pool.
func NewPromotionRepository(pool *pgxpool.Pool) *PromotionRepository {
	return &PromotionRepository{pool: pool}
}

// ensureProject distinguishes an empty project result from a missing project.
func ensureProject(ctx context.Context, queryer promotionRowQuerier, projectID int64) error {
	var exists bool
	if err := queryer.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM projects WHERE project_id = $1)`, projectID).Scan(&exists); err != nil {
		return fmt.Errorf("check project: %w", err)
	}

	if !exists {
		return ErrProjectNotFound
	}

	return nil
}

// List returns all promotions for a project, ordered by promotion ID. Each
// promotion and all of its items are hydrated by one ordered query.
func (repository *PromotionRepository) List(ctx context.Context, projectID int64) ([]models.ProjectPromotion, error) {
	if err := ensureProject(ctx, repository.pool, projectID); err != nil {
		return nil, err
	}

	return repository.queryPromotions(ctx, repository.pool, projectID, nil)
}

// Detail returns one promotion belonging to a project.
func (repository *PromotionRepository) Detail(ctx context.Context, projectID, promotionID int64) (models.ProjectPromotion, error) {
	if err := ensureProject(ctx, repository.pool, projectID); err != nil {
		return models.ProjectPromotion{}, err
	}

	promotions, err := repository.queryPromotions(ctx, repository.pool, projectID, &promotionID)
	if err != nil {
		return models.ProjectPromotion{}, err
	}
	if len(promotions) == 0 {
		return models.ProjectPromotion{}, ErrPromotionNotFound
	}

	return promotions[0], nil
}

// ResolveProjectProducts resolves all references with one query. The
// transaction-aware form used by Create and Update adds FOR UPDATE so the
// assignments cannot change between resolution and item insertion.
func (repository *PromotionRepository) ResolveProjectProducts(ctx context.Context, projectID int64, references []models.ProjectPromotionItemInput) ([]ProjectProductAssignment, error) {
	return resolveProjectProducts(ctx, repository.pool, projectID, references, false)
}

// Create inserts a promotion and its complete item set atomically. The input
// is expected to have passed use-case validation; database and arithmetic
// checks remain defensive here because this method also forms the transaction
// boundary for the mutation.
func (repository *PromotionRepository) Create(ctx context.Context, today models.Date, projectID int64, mutation models.ProjectPromotionMutation) (models.ProjectPromotion, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("begin promotion create: %w", err)
	}
	defer rollback(ctx, tx)

	if err := lockProjectForMutation(ctx, tx, today, projectID); err != nil {
		return models.ProjectPromotion{}, err
	}

	assignments, err := resolveProjectProducts(ctx, tx, projectID, mutation.Items, true)
	if err != nil {
		return models.ProjectPromotion{}, err
	}

	if err := validatePromotionPrice(mutation, assignments); err != nil {
		return models.ProjectPromotion{}, err
	}

	var promotionID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO promotions (project_id, name, promotion_price)
		VALUES ($1, $2, $3::numeric)
		RETURNING promotion_id`, projectID, mutation.Name, mutation.PromotionPrice.String()).Scan(&promotionID); err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("insert promotion: %w", err)
	}

	if err := insertPromotionItems(ctx, tx, promotionID, projectID, mutation.Items, assignments); err != nil {
		return models.ProjectPromotion{}, err
	}

	promotions, err := repository.queryPromotions(ctx, tx, projectID, &promotionID)
	if err != nil {
		return models.ProjectPromotion{}, err
	}
	if len(promotions) != 1 {
		return models.ProjectPromotion{}, fmt.Errorf("%w: created promotion %d could not be hydrated", ErrPromotionPricingCorrupt, promotionID)
	}

	if err := tx.Commit(ctx); err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("commit promotion create: %w", err)
	}

	return promotions[0], nil
}

// Update replaces a promotion's complete item set atomically.
func (repository *PromotionRepository) Update(ctx context.Context, today models.Date, projectID, promotionID int64, mutation models.ProjectPromotionMutation) (models.ProjectPromotion, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("begin promotion update: %w", err)
	}
	defer rollback(ctx, tx)

	if err := lockProjectForMutation(ctx, tx, today, projectID); err != nil {
		return models.ProjectPromotion{}, err
	}

	if err := lockPromotion(ctx, tx, projectID, promotionID); err != nil {
		return models.ProjectPromotion{}, err
	}

	assignments, err := resolveProjectProducts(ctx, tx, projectID, mutation.Items, true)
	if err != nil {
		return models.ProjectPromotion{}, err
	}

	if err := validatePromotionPrice(mutation, assignments); err != nil {
		return models.ProjectPromotion{}, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE promotions
		SET name = $3, promotion_price = $4::numeric, updated_at = CURRENT_TIMESTAMP
		WHERE promotion_id = $1 AND project_id = $2`, promotionID, projectID, mutation.Name, mutation.PromotionPrice.String()); err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("update promotion: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM promotion_items WHERE promotion_id = $1 AND project_id = $2`, promotionID, projectID); err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("replace promotion items: %w", err)
	}

	if err := insertPromotionItems(ctx, tx, promotionID, projectID, mutation.Items, assignments); err != nil {
		return models.ProjectPromotion{}, err
	}

	promotions, err := repository.queryPromotions(ctx, tx, projectID, &promotionID)
	if err != nil {
		return models.ProjectPromotion{}, err
	}
	if len(promotions) != 1 {
		return models.ProjectPromotion{}, fmt.Errorf("%w: updated promotion %d could not be hydrated", ErrPromotionPricingCorrupt, promotionID)
	}

	if err := tx.Commit(ctx); err != nil {
		return models.ProjectPromotion{}, fmt.Errorf("commit promotion update: %w", err)
	}

	return promotions[0], nil
}

// Delete removes a promotion after applying the same project-status mutation
// guard used by create and update. The database cascades its promotion items.
func (repository *PromotionRepository) Delete(ctx context.Context, today models.Date, projectID, promotionID int64) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin promotion delete: %w", err)
	}
	defer rollback(ctx, tx)

	if err := lockProjectForMutation(ctx, tx, today, projectID); err != nil {
		return err
	}

	if err := lockPromotion(ctx, tx, projectID, promotionID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM promotions WHERE promotion_id = $1 AND project_id = $2`, promotionID, projectID); err != nil {
		return fmt.Errorf("delete promotion: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit promotion delete: %w", err)
	}

	return nil
}

type promotionRowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type promotionQueryer interface {
	promotionRowQuerier
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func lockProjectForMutation(ctx context.Context, tx pgx.Tx, today models.Date, projectID int64) error {
	var lockedProjectID int64
	var startDate, endDate time.Time

	err := tx.QueryRow(ctx, `
		SELECT project_id, start_date, end_date
		FROM projects
		WHERE project_id = $1
		FOR UPDATE`, projectID).Scan(&lockedProjectID, &startDate, &endDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProjectNotFound
	}
	if err != nil {
		return fmt.Errorf("lock project for promotion mutation: %w", err)
	}

	status := models.ProjectStatusFor(models.NewDate(startDate), models.NewDate(endDate), today)
	if status == models.ProjectStatusCompleted {
		return ErrProjectCompleted
	}

	return nil
}

func lockPromotion(ctx context.Context, tx pgx.Tx, projectID, promotionID int64) error {
	var lockedPromotionID int64

	err := tx.QueryRow(ctx, `
		SELECT promotion_id
		FROM promotions
		WHERE promotion_id = $1 AND project_id = $2
		FOR UPDATE`, promotionID, projectID).Scan(&lockedPromotionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPromotionNotFound
	}
	if err != nil {
		return fmt.Errorf("lock promotion: %w", err)
	}

	return nil
}

func validatePromotionPrice(input models.ProjectPromotionMutation, assignments []ProjectProductAssignment) error {
	var bundlePrice models.THBAmount
	for index, assignment := range assignments {
		lineTotal, err := assignment.ProjectPrice.Mul(int64(input.Items[index].Quantity))
		if err != nil {
			return fmt.Errorf("%w: calculate item %d: %v", ErrPromotionPricingCorrupt, index, err)
		}

		bundlePrice, err = bundlePrice.Add(lineTotal)
		if err != nil {
			return fmt.Errorf("%w: calculate bundle price: %v", ErrPromotionPricingCorrupt, err)
		}
	}

	if input.PromotionPrice.Satang() > bundlePrice.Satang() {
		return ErrPromotionPriceExceedsBundle
	}

	return nil
}

func resolveProjectProducts(ctx context.Context, queryer promotionQueryer, projectID int64, references []models.ProjectPromotionItemInput, lockRows bool) ([]ProjectProductAssignment, error) {
	assignments := make([]ProjectProductAssignment, len(references))
	if len(references) == 0 {
		return assignments, nil
	}

	values := make([]string, 0, len(references))
	arguments := []any{projectID}
	for _, reference := range references {
		productParameter := len(arguments) + 1
		variantParameter := len(arguments) + 2
		values = append(values, fmt.Sprintf("($%d::bigint, $%d::bigint)", productParameter, variantParameter))
		arguments = append(arguments, reference.ProductID, reference.VariantID)
	}

	query := `
		SELECT pp.project_product_id, pp.project_id, pp.product_id,
		       pp.variant_id, pp.project_price::text
		FROM project_products pp
		JOIN (VALUES ` + strings.Join(values, ", ") + `) AS requested(product_id, variant_id)
		  ON pp.product_id = requested.product_id
		 AND pp.variant_id IS NOT DISTINCT FROM requested.variant_id
		WHERE pp.project_id = $1`
	if lockRows {
		query += ` FOR UPDATE`
	}

	rows, err := queryer.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("resolve project products: %w", err)
	}
	defer rows.Close()

	assignmentsByReference := make(map[projectProductReference]ProjectProductAssignment, len(references))
	for rows.Next() {
		var assignment ProjectProductAssignment
		if err := rows.Scan(&assignment.ProjectProductID, &assignment.ProjectID, &assignment.ProductID, &assignment.VariantID, &assignment.ProjectPrice); err != nil {
			return nil, fmt.Errorf("scan project product assignment: %w", err)
		}

		key := newProjectProductReference(assignment.ProductID, assignment.VariantID)
		if _, exists := assignmentsByReference[key]; exists {
			return nil, fmt.Errorf("%w: duplicate project-product assignment for product %d", ErrPromotionPricingCorrupt, assignment.ProductID)
		}

		assignmentsByReference[key] = assignment
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project product assignments: %w", err)
	}

	for index, reference := range references {
		key := newProjectProductReference(reference.ProductID, reference.VariantID)
		assignment, found := assignmentsByReference[key]
		if !found {
			return nil, fmt.Errorf("%w: product %d is not assigned to project %d", ErrProductNotSellable, reference.ProductID, projectID)
		}

		assignments[index] = assignment
	}

	return assignments, nil
}

type projectProductReference struct {
	productID       int64
	variantID       int64
	variantIDIsNull bool
}

func newProjectProductReference(productID int64, variantID *int64) projectProductReference {
	if variantID == nil {
		return projectProductReference{productID: productID, variantIDIsNull: true}
	}

	return projectProductReference{productID: productID, variantID: *variantID}
}

func insertPromotionItems(ctx context.Context, tx pgx.Tx, promotionID, projectID int64, items []models.ProjectPromotionItemInput, assignments []ProjectProductAssignment) error {
	if len(items) != len(assignments) {
		return fmt.Errorf("%w: item and assignment counts differ", ErrPromotionPricingCorrupt)
	}

	if len(items) == 0 {
		return nil
	}

	values := make([]string, 0, len(items))
	arguments := []any{promotionID, projectID}
	for index, item := range items {
		projectProductParameter := len(arguments) + 1
		quantityParameter := len(arguments) + 2
		values = append(values, fmt.Sprintf("($1, $2, $%d, $%d)", projectProductParameter, quantityParameter))
		arguments = append(arguments, assignments[index].ProjectProductID, item.Quantity)
	}

	query := `INSERT INTO promotion_items (promotion_id, project_id, project_product_id, required_quantity)
		VALUES ` + strings.Join(values, ", ")
	if _, err := tx.Exec(ctx, query, arguments...); err != nil {
		return fmt.Errorf("insert promotion items: %w", err)
	}

	return nil
}

func (repository *PromotionRepository) queryPromotions(ctx context.Context, queryer promotionQueryer, projectID int64, promotionID *int64) ([]models.ProjectPromotion, error) {
	query, arguments := promotionHydrationQuery(projectID, promotionID)
	rows, err := queryer.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query promotions: %w", err)
	}
	defer rows.Close()

	promotions, err := scanPromotionHydrationRows(rows)
	if err != nil {
		return nil, err
	}
	if err := calculatePromotionTotals(promotions); err != nil {
		return nil, err
	}
	return promotions, nil
}

func promotionHydrationQuery(projectID int64, promotionID *int64) (string, []any) {
	const baseQuery = `
		SELECT p.promotion_id, p.project_id, p.name, p.promotion_price::text,
		       p.created_at, p.updated_at,
		       pi.project_product_id, pi.required_quantity,
		       pp.product_id, pp.variant_id, pp.project_price::text,
		       product.name, variant.variant_id, variant.size, variant.color
		FROM promotions p
		LEFT JOIN promotion_items pi
		  ON pi.promotion_id = p.promotion_id AND pi.project_id = p.project_id
		LEFT JOIN project_products pp
		  ON pp.project_product_id = pi.project_product_id AND pp.project_id = pi.project_id
		LEFT JOIN products product ON product.id = pp.product_id
		LEFT JOIN variants variant
		  ON variant.variant_id = pp.variant_id AND variant.product_id = pp.product_id
		WHERE p.project_id = $1`

	query := baseQuery
	arguments := []any{projectID}
	if promotionID != nil {
		query += ` AND p.promotion_id = $2`
		arguments = append(arguments, *promotionID)
	}
	query += ` ORDER BY p.promotion_id ASC, pp.product_id ASC, pp.variant_id NULLS FIRST, pp.project_product_id ASC`

	return query, arguments
}

type promotionHydrationRow struct {
	PromotionID      int64
	ProjectID        int64
	Name             string
	PromotionPrice   string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ProjectProductID *int64
	RequiredQuantity *int32
	ProductID        *int64
	VariantID        *int64
	ProjectPrice     *string
	ProductName      *string
	VariantRowID     *int64
	Size             *string
	Color            *string
}

func scanPromotionHydrationRows(rows pgx.Rows) ([]models.ProjectPromotion, error) {
	promotions := make([]models.ProjectPromotion, 0)
	for rows.Next() {
		row, err := scanPromotionHydrationRow(rows)
		if err != nil {
			return nil, err
		}
		promotions, err = appendPromotionHydrationRow(promotions, row)
		if err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate promotions: %w", err)
	}
	return promotions, nil
}

func scanPromotionHydrationRow(rows pgx.Rows) (promotionHydrationRow, error) {
	var row promotionHydrationRow
	if err := rows.Scan(
		&row.PromotionID, &row.ProjectID, &row.Name, &row.PromotionPrice,
		&row.CreatedAt, &row.UpdatedAt,
		&row.ProjectProductID, &row.RequiredQuantity,
		&row.ProductID, &row.VariantID, &row.ProjectPrice,
		&row.ProductName, &row.VariantRowID, &row.Size, &row.Color,
	); err != nil {
		return promotionHydrationRow{}, fmt.Errorf("scan promotion: %w", err)
	}
	return row, nil
}

func appendPromotionHydrationRow(promotions []models.ProjectPromotion, row promotionHydrationRow) ([]models.ProjectPromotion, error) {
	promotionPrice, err := models.ParseTHBAmount(row.PromotionPrice)
	if err != nil {
		return nil, fmt.Errorf("%w: promotion %d price: %v", ErrPromotionPricingCorrupt, row.PromotionID, err)
	}
	if len(promotions) == 0 || promotions[len(promotions)-1].PromotionID != row.PromotionID {
		promotions = append(promotions, models.ProjectPromotion{
			PromotionID:    row.PromotionID,
			ProjectID:      row.ProjectID,
			Name:           row.Name,
			Items:          make([]models.ProjectPromotionItem, 0),
			PromotionPrice: promotionPrice,
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
		})
	}
	if row.ProjectProductID == nil {
		return promotions, nil
	}

	item, err := promotionItemFromHydrationRow(row)
	if err != nil {
		return nil, err
	}
	last := len(promotions) - 1
	promotions[last].Items = append(promotions[last].Items, item)
	return promotions, nil
}

func promotionItemFromHydrationRow(row promotionHydrationRow) (models.ProjectPromotionItem, error) {
	if row.RequiredQuantity == nil || row.ProductID == nil || row.ProjectPrice == nil || row.ProductName == nil {
		return models.ProjectPromotionItem{}, fmt.Errorf("%w: promotion %d contains an incomplete item", ErrPromotionPricingCorrupt, row.PromotionID)
	}
	if row.VariantID != nil && row.VariantRowID == nil {
		return models.ProjectPromotionItem{}, fmt.Errorf("%w: project product %d references a missing or unrelated variant", ErrPromotionPricingCorrupt, *row.ProjectProductID)
	}
	projectPrice, err := models.ParseTHBAmount(*row.ProjectPrice)
	if err != nil {
		return models.ProjectPromotionItem{}, fmt.Errorf("%w: project product %d price: %v", ErrPromotionPricingCorrupt, *row.ProjectProductID, err)
	}
	if *row.RequiredQuantity <= 0 {
		return models.ProjectPromotionItem{}, fmt.Errorf("%w: promotion %d has invalid quantity", ErrPromotionPricingCorrupt, row.PromotionID)
	}
	if row.VariantID == nil && row.VariantRowID != nil {
		return models.ProjectPromotionItem{}, fmt.Errorf("%w: variantless project product %d joined a variant", ErrPromotionPricingCorrupt, *row.ProjectProductID)
	}
	return models.ProjectPromotionItem{
		ProductID:   *row.ProductID,
		VariantID:   row.VariantID,
		Quantity:    *row.RequiredQuantity,
		ProductName: *row.ProductName,
		Size:        row.Size,
		Color:       row.Color,
		UnitPrice:   projectPrice,
	}, nil
}

func calculatePromotionTotals(promotions []models.ProjectPromotion) error {
	for index := range promotions {
		originalPrice, err := calculatePromotionBundlePrice(promotions[index])
		if err != nil {
			return err
		}
		discount, err := originalPrice.Sub(promotions[index].PromotionPrice)
		if err != nil {
			return fmt.Errorf("%w: promotion %d price exceeds current bundle price", ErrPromotionPricingCorrupt, promotions[index].PromotionID)
		}
		promotions[index].OriginalBundlePrice = originalPrice
		promotions[index].Discount = discount
	}
	return nil
}

func calculatePromotionBundlePrice(promotion models.ProjectPromotion) (models.THBAmount, error) {
	if len(promotion.Items) == 0 {
		return models.THBAmount{}, fmt.Errorf("%w: promotion %d has no items", ErrPromotionPricingCorrupt, promotion.PromotionID)
	}

	var originalPrice models.THBAmount
	for itemIndex, item := range promotion.Items {
		lineTotal, err := item.UnitPrice.Mul(int64(item.Quantity))
		if err != nil {
			return models.THBAmount{}, fmt.Errorf("%w: promotion %d item %d: %v", ErrPromotionPricingCorrupt, promotion.PromotionID, itemIndex, err)
		}
		originalPrice, err = originalPrice.Add(lineTotal)
		if err != nil {
			return models.THBAmount{}, fmt.Errorf("%w: promotion %d bundle price: %v", ErrPromotionPricingCorrupt, promotion.PromotionID, err)
		}
	}
	return originalPrice, nil
}
