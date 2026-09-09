package usecases

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

var (
	// ErrInvalidPOSCart reports malformed Cart input.
	ErrInvalidPOSCart = errors.New("invalid POS Cart")
	// ErrProjectNotActive reports a quotation requested outside an ACTIVE
	// project.
	ErrProjectNotActive = errors.New("project is not active")
	// ErrInsufficientStock reports one or more Cart quantities above live stock.
	ErrInsufficientStock = errors.New("insufficient stock")
)

// POSValidationError carries a client-facing message for a malformed POS
// Cart while retaining a stable sentinel for transport mapping.
type POSValidationError struct{ Message string }

// Error returns the client-facing validation message.
func (err POSValidationError) Error() string { return err.Message }

// Unwrap identifies this error as malformed POS Cart input.
func (err POSValidationError) Unwrap() error { return ErrInvalidPOSCart }

// POSInsufficientStockError reports every Cart line that conflicts with live
// stock, preserving request order for a deterministic API response.
type POSInsufficientStockError struct {
	Items []models.POSStockConflict
}

// Error returns the client-facing stock conflict message.
func (err POSInsufficientStockError) Error() string {
	return "requested quantity exceeds available stock"
}

// Unwrap identifies this error as an insufficient-stock conflict.
func (err POSInsufficientStockError) Unwrap() error { return ErrInsufficientStock }

// POSSnapshotReader is the persistence port required by POSService. The
// repository supplies both snapshots from one consistent database read.
type POSSnapshotReader interface {
	Catalog(context.Context, models.Date, int64) (models.POSCatalogSnapshot, error)
	QuoteSnapshot(context.Context, models.Date, int64, []models.POSCartItemRequest) (models.POSQuoteSnapshot, error)
}

// POSService validates POS requests, groups catalogue data, checks live stock,
// and delegates money calculations to the reusable PricingService.
type POSService struct {
	reader  POSSnapshotReader
	pricing *PricingService
	now     func() time.Time
}

// NewPOSService constructs a POS service over its snapshot reader.
func NewPOSService(reader POSSnapshotReader) *POSService {
	return &POSService{
		reader:  reader,
		pricing: NewPricingService(),
		now:     time.Now,
	}
}

// Catalog returns the selected Project Products grouped by category. Preview
// remains available for every project status; only ACTIVE projects can check
// out.
func (service *POSService) Catalog(ctx context.Context, projectID int64) (models.POSCatalog, error) {
	if projectID <= 0 {
		return models.POSCatalog{}, ErrInvalidProjectID
	}

	snapshot, err := service.reader.Catalog(ctx, models.TodayInBangkok(service.now()), projectID)
	if err != nil {
		return models.POSCatalog{}, fmt.Errorf("get POS catalogue: %w", err)
	}

	return models.POSCatalog{
		Project:     snapshot.Project,
		CanCheckout: snapshot.Project.Status == models.ProjectStatusActive,
		Categories:  groupPOSCategories(snapshot.Items),
	}, nil
}

// Quote validates and calculates a server-side quotation from current
// catalogue and Promotion snapshot data. Client requests contain identities
// and quantities only; all monetary values come from the snapshot.
func (service *POSService) Quote(ctx context.Context, projectID int64, request models.POSCartRequest) (models.POSQuote, error) {
	if projectID <= 0 {
		return models.POSQuote{}, ErrInvalidProjectID
	}
	if err := validatePOSCartRequest(request); err != nil {
		return models.POSQuote{}, err
	}

	now := service.now()
	snapshot, err := service.reader.QuoteSnapshot(ctx, models.TodayInBangkok(now), projectID, request.Items)
	if err != nil {
		return models.POSQuote{}, fmt.Errorf("read POS quote snapshot: %w", err)
	}

	if snapshot.Project.Status != models.ProjectStatusActive {
		return models.POSQuote{}, ErrProjectNotActive
	}

	if conflicts := findPOSStockConflicts(snapshot.Items); len(conflicts) != 0 {
		return models.POSQuote{}, POSInsufficientStockError{Items: conflicts}
	}

	pricingCart := pricingCartFromSnapshot(snapshot.Items)
	pricingResult, err := service.pricing.Calculate(pricingCart, snapshot.Promotions)
	if err != nil {
		return models.POSQuote{}, fmt.Errorf("calculate POS quote: %w", err)
	}

	lineItems, err := quoteLineItemsFromSnapshot(snapshot.Items)
	if err != nil {
		return models.POSQuote{}, err
	}

	return models.POSQuote{
		ProjectID:        snapshot.Project.ProjectID,
		Items:            lineItems,
		Subtotal:         pricingResult.Subtotal,
		AppliedPromotion: pricingResult.AppliedPromotion,
		Discount:         pricingResult.Discount,
		NetTotal:         pricingResult.NetTotal,
		QuotedAt:         now,
	}, nil
}

func validatePOSCartRequest(request models.POSCartRequest) error {
	if len(request.Items) == 0 {
		return POSValidationError{Message: "Cart must contain at least one item"}
	}

	seen := make(map[posCartItemKey]struct{}, len(request.Items))
	for index, item := range request.Items {
		if item.ProductID <= 0 {
			return POSValidationError{Message: fmt.Sprintf("Cart item %d product ID must be positive", index+1)}
		}

		if item.VariantID != nil && *item.VariantID <= 0 {
			return POSValidationError{Message: fmt.Sprintf("Cart item %d variant ID must be positive", index+1)}
		}

		if item.Quantity <= 0 {
			return POSValidationError{Message: fmt.Sprintf("Cart item %d quantity must be positive", index+1)}
		}

		key := newPOSCartItemKey(item.ProductID, item.VariantID)
		if _, exists := seen[key]; exists {
			return POSValidationError{Message: fmt.Sprintf("Cart items must not contain duplicate product/variant reference at item %d", index+1)}
		}
		seen[key] = struct{}{}
	}

	return nil
}

type posCartItemKey struct {
	productID       int64
	variantID       int64
	variantIDIsNull bool
}

func newPOSCartItemKey(productID int64, variantID *int64) posCartItemKey {
	if variantID == nil {
		return posCartItemKey{productID: productID, variantIDIsNull: true}
	}

	return posCartItemKey{productID: productID, variantID: *variantID}
}

type posCategoryGroup struct {
	name     *string
	products []models.ProjectProductAssignment
}

func groupPOSCategories(items []models.POSResolvedItem) []models.POSCategory {
	named := make(map[string]*posCategoryGroup)
	var uncategorized *posCategoryGroup

	for _, item := range items {
		product := projectProductFromPOSItem(item)
		if item.Category == nil {
			if uncategorized == nil {
				uncategorized = &posCategoryGroup{products: make([]models.ProjectProductAssignment, 0)}
			}
			uncategorized.products = append(uncategorized.products, product)
			continue
		}

		categoryName := *item.Category
		group, exists := named[categoryName]
		if !exists {
			name := categoryName
			group = &posCategoryGroup{
				name:     &name,
				products: make([]models.ProjectProductAssignment, 0),
			}
			named[categoryName] = group
		}
		group.products = append(group.products, product)
	}

	categories := make([]models.POSCategory, 0, len(named)+1)
	for _, group := range named {
		categories = append(categories, models.POSCategory{
			Name:     clonePOSString(group.name),
			Products: group.products,
		})
	}

	sort.SliceStable(categories, func(left, right int) bool {
		leftLower := strings.ToLower(*categories[left].Name)
		rightLower := strings.ToLower(*categories[right].Name)
		if leftLower == rightLower {
			return *categories[left].Name < *categories[right].Name
		}
		return leftLower < rightLower
	})

	if uncategorized != nil {
		categories = append(categories, models.POSCategory{
			Name:     nil,
			Products: uncategorized.products,
		})
	}

	return categories
}

func projectProductFromPOSItem(item models.POSResolvedItem) models.ProjectProductAssignment {
	return models.ProjectProductAssignment{
		ProductID:     item.ProductID,
		VariantID:     clonePOSInt64(item.VariantID),
		ProductName:   item.ProductName,
		Category:      clonePOSString(item.Category),
		ImageURL:      clonePOSString(item.ImageURL),
		Size:          clonePOSString(item.Size),
		Color:         clonePOSString(item.Color),
		StockQuantity: item.StockQuantity,
		ProjectPrice:  item.ProjectPrice.String(),
	}
}

func pricingCartFromSnapshot(items []models.POSResolvedCartItem) []models.PricingCartLine {
	pricingCart := make([]models.PricingCartLine, len(items))
	for index, item := range items {
		pricingCart[index] = models.PricingCartLine{
			ProductID: item.Item.ProductID,
			VariantID: clonePOSInt64(item.Item.VariantID),
			Quantity:  item.Quantity,
			UnitPrice: item.Item.ProjectPrice,
		}
	}

	return pricingCart
}

func findPOSStockConflicts(items []models.POSResolvedCartItem) []models.POSStockConflict {
	conflicts := make([]models.POSStockConflict, 0)
	for _, item := range items {
		if item.Quantity <= item.Item.StockQuantity {
			continue
		}

		conflicts = append(conflicts, models.POSStockConflict{
			ProductID:         item.Item.ProductID,
			VariantID:         clonePOSInt64(item.Item.VariantID),
			RequestedQuantity: item.Quantity,
			AvailableQuantity: item.Item.StockQuantity,
		})
	}

	return conflicts
}

func quoteLineItemsFromSnapshot(items []models.POSResolvedCartItem) ([]models.POSQuoteLineItem, error) {
	lineItems := make([]models.POSQuoteLineItem, len(items))
	for index, item := range items {
		lineTotal, err := item.Item.ProjectPrice.Mul(int64(item.Quantity))
		if err != nil {
			return nil, fmt.Errorf("calculate Cart item %d total: %w", index+1, err)
		}

		lineItems[index] = models.POSQuoteLineItem{
			ProductID:         item.Item.ProductID,
			VariantID:         clonePOSInt64(item.Item.VariantID),
			ProductName:       item.Item.ProductName,
			Size:              clonePOSString(item.Item.Size),
			Color:             clonePOSString(item.Item.Color),
			ImageURL:          clonePOSString(item.Item.ImageURL),
			Quantity:          item.Quantity,
			UnitPrice:         item.Item.ProjectPrice,
			LineTotal:         lineTotal,
			AvailableQuantity: item.Item.StockQuantity,
		}
	}

	return lineItems, nil
}

func clonePOSInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}

func clonePOSString(value *string) *string {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}
