package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrPOSDataCorrupt reports persisted catalogue data that cannot be mapped
	// to the money-safe POS snapshot model.
	ErrPOSDataCorrupt = errors.New("POS data is corrupt")
	// ErrPOSDuplicateCartItem reports a duplicate identity passed directly to
	// the repository. The POS use case performs the same validation before it
	// reaches this boundary.
	ErrPOSDuplicateCartItem = errors.New("duplicate POS cart item")
)

// POSRepository reads the project catalogue and quotation inputs. It does not
// calculate prices; the resolved quote snapshot is consumed by the pure
// usecases.PricingService.
type POSRepository struct {
	pool       *pgxpool.Pool
	promotions *PromotionRepository
}

// NewPOSRepository constructs a POS repository over the connection pool.
func NewPOSRepository(pool *pgxpool.Pool) *POSRepository {
	return &POSRepository{
		pool:       pool,
		promotions: NewPromotionRepository(pool),
	}
}

// posResolvedItemColumns is shared by catalogue and request-resolution
// queries. The variant join includes product_id so an invalid cross-product
// variant reference cannot silently hydrate as a different sellable item.
const posResolvedItemColumns = `pp.product_id, pp.variant_id,
       p.name, p.category, p.images[1],
       v.size, v.color, v.variant_id,
       ` + `COALESCE(CASE WHEN pp.variant_id IS NULL THEN p.stock_quantity ELSE v.stock_quantity END, 0),
       pp.project_price::text`

const posResolvedItemJoins = `
JOIN products p ON p.id = pp.product_id
LEFT JOIN variants v
  ON v.variant_id = pp.variant_id AND v.product_id = pp.product_id`

const posCatalogueItemsQuery = `SELECT ` + posResolvedItemColumns + `
FROM project_products pp` + posResolvedItemJoins + `
WHERE pp.project_id = $1
ORDER BY pp.product_id, pp.variant_id NULLS FIRST, pp.project_product_id`

// Catalog returns all selected Project Products, including for NOT_STARTED and
// COMPLETED projects. The caller derives can_checkout from Project.Status.
func (repository *POSRepository) Catalog(ctx context.Context, today models.Date, projectID int64) (models.POSCatalogSnapshot, error) {
	tx, err := repository.beginReadSnapshot(ctx)
	if err != nil {
		return models.POSCatalogSnapshot{}, err
	}
	defer rollback(ctx, tx)

	project, err := readPOSProject(ctx, tx, today, projectID)
	if err != nil {
		return models.POSCatalogSnapshot{}, err
	}

	items, err := queryPOSCatalogueItems(ctx, tx, projectID)
	if err != nil {
		return models.POSCatalogSnapshot{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.POSCatalogSnapshot{}, fmt.Errorf("commit POS catalogue snapshot: %w", err)
	}

	return models.POSCatalogSnapshot{Project: project, Items: items}, nil
}

// QuoteSnapshot resolves the requested Cart identities and hydrates all
// project Promotions from one repeatable-read, read-only snapshot. Quantity is
// retained for the pricing use case, while prices and stock are always read
// from project_products/products/variants.
func (repository *POSRepository) QuoteSnapshot(ctx context.Context, today models.Date, projectID int64, cart []models.POSCartItemRequest) (models.POSQuoteSnapshot, error) {
	if err := validatePOSCartIdentities(cart); err != nil {
		return models.POSQuoteSnapshot{}, err
	}

	tx, err := repository.beginReadSnapshot(ctx)
	if err != nil {
		return models.POSQuoteSnapshot{}, err
	}
	defer rollback(ctx, tx)

	project, err := readPOSProject(ctx, tx, today, projectID)
	if err != nil {
		return models.POSQuoteSnapshot{}, err
	}

	items, err := queryPOSCartItems(ctx, tx, projectID, cart)
	if err != nil {
		return models.POSQuoteSnapshot{}, err
	}

	promotionRepository := repository.promotions
	if promotionRepository == nil {
		// Keep a zero-value POSRepository useful in package-local tests while
		// preserving the constructor's shared pool in normal application code.
		promotionRepository = NewPromotionRepository(repository.pool)
	}

	hydratedPromotions, err := promotionRepository.queryPromotions(ctx, tx, projectID, nil)
	if err != nil {
		return models.POSQuoteSnapshot{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.POSQuoteSnapshot{}, fmt.Errorf("commit POS quote snapshot: %w", err)
	}

	return models.POSQuoteSnapshot{
		Project:    project,
		Items:      items,
		Promotions: pricingPromotionsFromHydrated(hydratedPromotions),
	}, nil
}

func (repository *POSRepository) beginReadSnapshot(ctx context.Context) (pgx.Tx, error) {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("begin POS read snapshot: %w", err)
	}
	return tx, nil
}

func readPOSProject(ctx context.Context, tx pgx.Tx, today models.Date, projectID int64) (models.Project, error) {
	project, err := scanProject(tx.QueryRow(ctx, detailProjectQuery, today.Time, projectID))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Project{}, ErrProjectNotFound
	}
	if err != nil {
		return models.Project{}, fmt.Errorf("get POS project: %w", err)
	}

	return project, nil
}

func queryPOSCatalogueItems(ctx context.Context, queryer promotionQueryer, projectID int64) ([]models.POSResolvedItem, error) {
	rows, err := queryer.Query(ctx, posCatalogueItemsQuery, projectID)
	if err != nil {
		return nil, fmt.Errorf("query POS catalogue items: %w", err)
	}
	defer rows.Close()

	items := make([]models.POSResolvedItem, 0)
	for rows.Next() {
		item, err := scanPOSResolvedItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate POS catalogue items: %w", err)
	}

	return items, nil
}

func queryPOSCartItems(ctx context.Context, queryer promotionQueryer, projectID int64, cart []models.POSCartItemRequest) ([]models.POSResolvedCartItem, error) {
	items := make([]models.POSResolvedCartItem, len(cart))
	if len(cart) == 0 {
		return items, nil
	}

	requestedTable, arguments := buildPOSRequestedTable(projectID, cart)

	query := `SELECT requested.ordinal, requested.quantity, ` + posResolvedItemColumns + `
FROM ` + requestedTable + `
JOIN project_products pp
  ON pp.project_id = $1
 AND pp.product_id = requested.product_id
 AND pp.variant_id IS NOT DISTINCT FROM requested.variant_id` + posResolvedItemJoins + `
ORDER BY requested.ordinal`

	rows, err := queryer.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query POS Cart items: %w", err)
	}
	defer rows.Close()

	matched := make([]bool, len(cart))

	for rows.Next() {
		var ordinal int64
		var quantity int32
		item, err := scanPOSResolvedItem(rows, &ordinal, &quantity)
		if err != nil {
			return nil, err
		}
		if ordinal < 0 || ordinal >= int64(len(cart)) {
			return nil, fmt.Errorf("%w: resolved Cart ordinal %d is outside the request", ErrPOSDataCorrupt, ordinal)
		}
		if matched[ordinal] {
			return nil, fmt.Errorf("%w: Cart ordinal %d resolved more than once", ErrPOSDataCorrupt, ordinal)
		}
		matched[ordinal] = true
		items[ordinal] = models.POSResolvedCartItem{Item: item, Quantity: quantity}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate POS Cart items: %w", err)
	}

	for index, found := range matched {
		if found {
			continue
		}
		item := cart[index]
		return nil, fmt.Errorf("%w: product %d is not assigned to project %d", ErrProductNotSellable, item.ProductID, projectID)
	}

	return items, nil
}

func scanPOSResolvedItem(row rowScanner, prefix ...any) (models.POSResolvedItem, error) {
	var (
		item            models.POSResolvedItem
		joinedVariantID *int64
		projectPriceRaw string
	)

	destinations := append(
		prefix,
		&item.ProductID,
		&item.VariantID,
		&item.ProductName,
		&item.Category,
		&item.ImageURL,
		&item.Size,
		&item.Color,
		&joinedVariantID,
		&item.StockQuantity,
		&projectPriceRaw,
	)
	if err := row.Scan(destinations...); err != nil {
		return models.POSResolvedItem{}, fmt.Errorf("scan POS item: %w", err)
	}

	if item.VariantID != nil && joinedVariantID == nil {
		return models.POSResolvedItem{}, fmt.Errorf("%w: product %d references missing or unrelated variant %d", ErrPOSDataCorrupt, item.ProductID, *item.VariantID)
	}

	projectPrice, err := models.ParseTHBAmount(projectPriceRaw)
	if err != nil {
		return models.POSResolvedItem{}, fmt.Errorf("%w: product %d project price: %v", ErrPOSDataCorrupt, item.ProductID, err)
	}

	item.ProjectPrice = projectPrice
	return item, nil
}

// buildPOSRequestedTable returns a parameterized, ordinal-preserving VALUES
// relation. The project ID remains $1 so the same query shape can be reused
// for every Cart size without interpolating client data.
func buildPOSRequestedTable(projectID int64, cart []models.POSCartItemRequest) (string, []any) {
	values := make([]string, 0, len(cart))
	arguments := []any{projectID}

	for index, item := range cart {
		ordinalParameter := len(arguments) + 1
		productParameter := len(arguments) + 2
		variantParameter := len(arguments) + 3
		quantityParameter := len(arguments) + 4
		values = append(values, fmt.Sprintf("($%d::bigint, $%d::bigint, $%d::bigint, $%d::integer)", ordinalParameter, productParameter, variantParameter, quantityParameter))
		arguments = append(arguments, index, item.ProductID, item.VariantID, item.Quantity)
	}

	return `(VALUES ` + strings.Join(values, ", ") + `) AS requested(ordinal, product_id, variant_id, quantity)`, arguments
}

func validatePOSCartIdentities(cart []models.POSCartItemRequest) error {
	seen := make(map[projectProductReference]struct{}, len(cart))

	for _, item := range cart {
		key := newProjectProductReference(item.ProductID, item.VariantID)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%w: product %d appears more than once", ErrPOSDuplicateCartItem, item.ProductID)
		}
		seen[key] = struct{}{}
	}

	return nil
}

func pricingPromotionsFromHydrated(promotions []models.ProjectPromotion) []models.PricingPromotion {
	converted := make([]models.PricingPromotion, len(promotions))

	for promotionIndex, promotion := range promotions {
		hydratedGroups := promotion.ItemGroups
		legacyItems := len(hydratedGroups) == 0 && len(promotion.Items) > 0
		if len(hydratedGroups) == 0 && len(promotion.Items) > 0 {
			hydratedGroups = make([]models.ProjectPromotionItemGroup, len(promotion.Items))
			for index, item := range promotion.Items {
				hydratedGroups[index] = models.ProjectPromotionItemGroup{Options: []models.ProjectPromotionItem{item}}
			}
		}
		groups := make([]models.PricingPromotionItemGroup, len(hydratedGroups))
		for groupIndex, group := range hydratedGroups {
			options := make([]models.PricingPromotionItem, len(group.Options))
			for optionIndex, item := range group.Options {
				var variantID *int64
				if item.VariantID != nil {
					value := *item.VariantID
					variantID = &value
				}

				options[optionIndex] = models.PricingPromotionItem{
					ProductID: item.ProductID,
					VariantID: variantID,
					Quantity:  item.Quantity,
				}
			}
			groups[groupIndex] = models.PricingPromotionItemGroup{Options: options}
		}

		converted[promotionIndex] = models.PricingPromotion{
			PromotionID:    promotion.PromotionID,
			Name:           promotion.Name,
			PromotionPrice: promotion.PromotionPrice,
		}
		if legacyItems {
			converted[promotionIndex].Items = flattenPricingPromotionGroups(groups)
		} else {
			converted[promotionIndex].ItemGroups = groups
		}
	}

	return converted
}

func flattenPricingPromotionGroups(groups []models.PricingPromotionItemGroup) []models.PricingPromotionItem {
	items := make([]models.PricingPromotionItem, 0)
	for _, group := range groups {
		items = append(items, group.Options...)
	}
	return items
}
