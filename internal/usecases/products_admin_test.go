package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
)

type productAdminStoreStub struct {
	archiveProductID int64
	archiveToday     models.Date
	archiveErr       error
}

func (stub *productAdminStoreStub) Create(context.Context, models.ProductInput) (models.ProductDetail, error) {
	return models.ProductDetail{}, nil
}
func (stub *productAdminStoreStub) Update(context.Context, int64, models.ProductInput) (models.ProductDetail, error) {
	return models.ProductDetail{}, nil
}
func (stub *productAdminStoreStub) Archive(_ context.Context, today models.Date, productID int64) error {
	stub.archiveToday = today
	stub.archiveProductID = productID
	return stub.archiveErr
}
func (stub *productAdminStoreStub) CreateVariant(context.Context, int64, models.VariantInput) (models.Variant, error) {
	return models.Variant{}, nil
}
func (stub *productAdminStoreStub) UpdateVariant(context.Context, int64, models.VariantInput) (models.Variant, error) {
	return models.Variant{}, nil
}
func (stub *productAdminStoreStub) DeleteVariant(context.Context, int64) error { return nil }

func TestProductAdminDeleteArchivesUsingBangkokDate(t *testing.T) {
	store := &productAdminStoreStub{}
	service := NewProductAdminService(store)
	// This instant is still the previous calendar day in Bangkok. Archive
	// eligibility must use Bangkok dates just like project status does.
	service.now = func() time.Time {
		return time.Date(2026, 10, 13, 18, 0, 0, 0, time.UTC)
	}

	if err := service.Delete(context.Background(), 9); err != nil {
		t.Fatal(err)
	}
	if store.archiveProductID != 9 {
		t.Fatalf("archive product ID = %d, want 9", store.archiveProductID)
	}
	if got, want := store.archiveToday.Time.Format("2006-01-02"), "2026-10-14"; got != want {
		t.Fatalf("archive date = %s, want %s", got, want)
	}
}

func TestProductAdminDeleteRejectsInvalidIDBeforeArchiving(t *testing.T) {
	store := &productAdminStoreStub{}
	if err := NewProductAdminService(store).Delete(context.Background(), 0); err == nil {
		t.Fatal("expected validation error")
	}
	if store.archiveProductID != 0 {
		t.Fatalf("archive unexpectedly called for product %d", store.archiveProductID)
	}
}
