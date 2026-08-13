package usecases

import (
	"context"
	"fmt"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

type ContentAdminStore interface {
	CreatePromo(context.Context, models.ContentInput) (models.Promo, error)
	UpdatePromo(context.Context, int64, models.ContentInput) (models.Promo, error)
	DeletePromo(context.Context, int64) error
	CreateBanner(context.Context, models.ContentInput) (models.Banner, error)
	UpdateBanner(context.Context, int64, models.ContentInput) (models.Banner, error)
	DeleteBanner(context.Context, int64) error
}

type ContentAdminService struct{ store ContentAdminStore }

func NewContentAdminService(store ContentAdminStore) *ContentAdminService {
	return &ContentAdminService{store: store}
}

func (s *ContentAdminService) CreatePromo(ctx context.Context, input models.ContentInput) (models.Promo, error) {
	if input.ImgURL == nil || *input.ImgURL == "" {
		return models.Promo{}, fmt.Errorf("image URL is required")
	}
	return s.store.CreatePromo(ctx, input)
}

func (s *ContentAdminService) UpdatePromo(ctx context.Context, id int64, input models.ContentInput) (models.Promo, error) {
	if id <= 0 {
		return models.Promo{}, fmt.Errorf("promo ID must be positive")
	}
	return s.store.UpdatePromo(ctx, id, input)
}

func (s *ContentAdminService) DeletePromo(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("promo ID must be positive")
	}
	return s.store.DeletePromo(ctx, id)
}

func (s *ContentAdminService) CreateBanner(ctx context.Context, input models.ContentInput) (models.Banner, error) {
	if input.ImgURL == nil || *input.ImgURL == "" {
		return models.Banner{}, fmt.Errorf("image URL is required")
	}
	return s.store.CreateBanner(ctx, input)
}

func (s *ContentAdminService) UpdateBanner(ctx context.Context, id int64, input models.ContentInput) (models.Banner, error) {
	if id <= 0 {
		return models.Banner{}, fmt.Errorf("banner ID must be positive")
	}
	return s.store.UpdateBanner(ctx, id, input)
}

func (s *ContentAdminService) DeleteBanner(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("banner ID must be positive")
	}
	return s.store.DeleteBanner(ctx, id)
}
