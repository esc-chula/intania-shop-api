package repositories

import (
	"context"
	"fmt"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ContentRepository struct{ pool *pgxpool.Pool }

func NewContentRepository(pool *pgxpool.Pool) *ContentRepository {
	return &ContentRepository{pool: pool}
}

func (r *ContentRepository) Promos(ctx context.Context, activeOnly bool) ([]models.Promo, error) {
	q := `SELECT promo_id,img_url,link_url,is_active,created_at FROM promos`
	if activeOnly {
		q += ` WHERE is_active = TRUE`
	}
	q += ` ORDER BY promo_id DESC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list promos: %w", err)
	}
	defer rows.Close()
	out := []models.Promo{}
	for rows.Next() {
		var v models.Promo
		if err := rows.Scan(&v.PromoID, &v.ImgURL, &v.LinkURL, &v.IsActive, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan promo: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate promos: %w", err)
	}
	return out, nil
}

func (r *ContentRepository) Banners(ctx context.Context, activeOnly bool) ([]models.Banner, error) {
	q := `SELECT banner_id,img_url,link_url,is_active,created_at FROM banners`
	if activeOnly {
		q += ` WHERE is_active = TRUE`
	}
	q += ` ORDER BY banner_id DESC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list banners: %w", err)
	}
	defer rows.Close()
	out := []models.Banner{}
	for rows.Next() {
		var v models.Banner
		if err := rows.Scan(&v.BannerID, &v.ImgURL, &v.LinkURL, &v.IsActive, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan banner: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate banners: %w", err)
	}
	return out, nil
}

func (r *ContentRepository) Promo(ctx context.Context, id int64) (models.Promo, error) {
	var out models.Promo
	err := r.pool.QueryRow(ctx, `SELECT promo_id,img_url,link_url,is_active,created_at FROM promos WHERE promo_id=$1`, id).Scan(&out.PromoID, &out.ImgURL, &out.LinkURL, &out.IsActive, &out.CreatedAt)
	if err != nil {
		return models.Promo{}, fmt.Errorf("get promo: %w", err)
	}
	return out, nil
}

func (r *ContentRepository) Banner(ctx context.Context, id int64) (models.Banner, error) {
	var out models.Banner
	err := r.pool.QueryRow(ctx, `SELECT banner_id,img_url,link_url,is_active,created_at FROM banners WHERE banner_id=$1`, id).Scan(&out.BannerID, &out.ImgURL, &out.LinkURL, &out.IsActive, &out.CreatedAt)
	if err != nil {
		return models.Banner{}, fmt.Errorf("get banner: %w", err)
	}
	return out, nil
}
