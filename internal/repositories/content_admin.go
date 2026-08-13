package repositories

import (
	"context"
	"fmt"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

func (r *ContentRepository) CreatePromo(ctx context.Context, input models.ContentInput) (models.Promo, error) {
	if input.ImgURL == nil {
		return models.Promo{}, fmt.Errorf("image URL is required")
	}
	var value models.Promo
	active := true
	if input.IsActive != nil {
		active = *input.IsActive
	}
	err := r.pool.QueryRow(ctx, `INSERT INTO promos(img_url,link_url,is_active) VALUES($1,$2,$3) RETURNING promo_id,img_url,link_url,is_active,created_at`, *input.ImgURL, input.LinkURL, active).Scan(&value.PromoID, &value.ImgURL, &value.LinkURL, &value.IsActive, &value.CreatedAt)
	if err != nil {
		return models.Promo{}, fmt.Errorf("create promo: %w", err)
	}
	return value, nil
}

func (r *ContentRepository) DeletePromo(ctx context.Context, id int64) error {
	tag, e := r.pool.Exec(ctx, `DELETE FROM promos WHERE promo_id=$1`, id)
	if e != nil {
		return fmt.Errorf("delete promo: %w", e)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("promo not found")
	}
	return nil
}

func (r *ContentRepository) CreateBanner(ctx context.Context, input models.ContentInput) (models.Banner, error) {
	if input.ImgURL == nil {
		return models.Banner{}, fmt.Errorf("image URL is required")
	}
	var value models.Banner
	active := true
	if input.IsActive != nil {
		active = *input.IsActive
	}
	err := r.pool.QueryRow(ctx, `INSERT INTO banners(img_url,link_url,is_active) VALUES($1,$2,$3) RETURNING banner_id,img_url,link_url,is_active,created_at`, *input.ImgURL, input.LinkURL, active).Scan(&value.BannerID, &value.ImgURL, &value.LinkURL, &value.IsActive, &value.CreatedAt)
	if err != nil {
		return models.Banner{}, fmt.Errorf("create banner: %w", err)
	}
	return value, nil
}

func (r *ContentRepository) DeleteBanner(ctx context.Context, id int64) error {
	tag, e := r.pool.Exec(ctx, `DELETE FROM banners WHERE banner_id=$1`, id)
	if e != nil {
		return fmt.Errorf("delete banner: %w", e)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("banner not found")
	}
	return nil
}

func (r *ContentRepository) UpdatePromo(ctx context.Context, id int64, input models.ContentInput) (models.Promo, error) {
	var out models.Promo
	err := r.pool.QueryRow(ctx, `UPDATE promos SET img_url=COALESCE($2,img_url),link_url=COALESCE($3,link_url),is_active=COALESCE($4,is_active) WHERE promo_id=$1 RETURNING promo_id,img_url,link_url,is_active,created_at`, id, input.ImgURL, input.LinkURL, input.IsActive).Scan(&out.PromoID, &out.ImgURL, &out.LinkURL, &out.IsActive, &out.CreatedAt)
	if err != nil {
		return models.Promo{}, fmt.Errorf("update promo: %w", err)
	}
	return out, nil
}

func (r *ContentRepository) UpdateBanner(ctx context.Context, id int64, input models.ContentInput) (models.Banner, error) {
	var out models.Banner
	err := r.pool.QueryRow(ctx, `UPDATE banners SET img_url=COALESCE($2,img_url),link_url=COALESCE($3,link_url),is_active=COALESCE($4,is_active) WHERE banner_id=$1 RETURNING banner_id,img_url,link_url,is_active,created_at`, id, input.ImgURL, input.LinkURL, input.IsActive).Scan(&out.BannerID, &out.ImgURL, &out.LinkURL, &out.IsActive, &out.CreatedAt)
	if err != nil {
		return models.Banner{}, fmt.Errorf("update banner: %w", err)
	}
	return out, nil
}
