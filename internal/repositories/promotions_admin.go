package repositories

import (
	"context"
	"fmt"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

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
