package usecases

import (
	"context"
	"github.com/esc-chula/intania-shop-api/internal/models"
)

type ContentReader interface {
	Promos(context.Context, bool) ([]models.Promo, error)
	Banners(context.Context, bool) ([]models.Banner, error)
	Promo(context.Context, int64) (models.Promo, error)
	Banner(context.Context, int64) (models.Banner, error)
}

type ContentService struct{ content ContentReader }

func NewContentService(content ContentReader) *ContentService {
	return &ContentService{content: content}
}

func (s *ContentService) Promos(ctx context.Context, active bool) ([]models.Promo, error) {
	return s.content.Promos(ctx, active)
}

func (s *ContentService) Banners(ctx context.Context, active bool) ([]models.Banner, error) {
	return s.content.Banners(ctx, active)
}

func (s *ContentService) Promo(ctx context.Context, id int64) (models.Promo, error) {
	return s.content.Promo(ctx, id)
}

func (s *ContentService) Banner(ctx context.Context, id int64) (models.Banner, error) {
	return s.content.Banner(ctx, id)
}
