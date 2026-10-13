//go:build integration

package repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
)

func TestProjectStatusOverridePersistenceFilteringAndAutomaticFallback(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	repository := NewProjectRepository(database)
	ctx := context.Background()
	today, _ := models.ParseDate("2026-10-13")
	start, _ := models.ParseDate("2026-10-13")
	end, _ := models.ParseDate("2026-10-14")
	name := "Override persistence"
	active := models.ProjectStatusActive
	input := models.ProjectInput{Name: &name, StartDate: &start, EndDate: &end, StatusOverride: &active}
	created, err := repository.Create(ctx, today, input)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := repository.Delete(ctx, created.ProjectID); err != nil {
			t.Errorf("cleanup project: %v", err)
		}
	})
	if created.Status != active || created.StatusOverride == nil || *created.StatusOverride != active {
		t.Fatalf("created project = %+v", created)
	}
	for _, date := range []string{"2026-10-12", "2026-10-13", "2026-10-14", "2026-10-15"} {
		day, _ := models.ParseDate(date)
		for _, status := range []models.ProjectStatus{models.ProjectStatusNotStarted, models.ProjectStatusActive, models.ProjectStatusCompleted} {
			input.StatusOverride = &status
			updated, err := repository.Update(ctx, day, created.ProjectID, input)
			if err != nil || updated.Status != status || updated.StatusOverride == nil || *updated.StatusOverride != status {
				t.Fatalf("update %s/%s = %+v, error = %v", date, status, updated, err)
			}
			projects, total, err := repository.List(ctx, day, models.ProjectFilter{Name: name, Status: status}, 0, 10)
			if err != nil || total != 1 || len(projects) != 1 || projects[0].Status != status {
				t.Fatalf("filter %s/%s = %+v, total = %d, error = %v", date, status, projects, total, err)
			}
			// Exercise the separate count query used for a page beyond the results.
			projects, total, err = repository.List(ctx, day, models.ProjectFilter{Name: name, Status: status}, 10, 10)
			if err != nil || total != 1 || len(projects) != 0 {
				t.Fatalf("empty page total = %d, error = %v", total, err)
			}
		}
		input.StatusOverride = nil
		updated, err := repository.Update(ctx, day, created.ProjectID, input)
		want := models.ProjectStatusFor(start, end, day)
		if err != nil || updated.Status != want || updated.StatusOverride != nil {
			t.Fatalf("automatic %s = %+v, error = %v, want %s", date, updated, err, want)
		}
	}
	if _, err := database.Exec(ctx, `UPDATE projects SET status_override = 'INVALID' WHERE project_id = $1`, created.ProjectID); err == nil {
		t.Fatal("database accepted an invalid status override")
	}
}

func TestProjectStatusOverrideControlsSalesAndMutationsOutsidePlannedDates(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	ctx := context.Background()
	for _, offset := range []int{-5, 5} {
		name := "past dates"
		if offset > 0 {
			name = "future dates"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newPromotionIntegrationFixture(t, database)
			projects := NewProjectRepository(database)
			pos := usecases.NewPOSService(NewPOSRepository(database))
			promotions := NewPromotionRepository(database)
			products := NewProductRepository(database)
			start := models.NewDate(fixture.Today.Time.AddDate(0, 0, offset))
			end := models.NewDate(start.Time.AddDate(0, 0, 1))
			active := models.ProjectStatusActive
			input := models.ProjectInput{Name: &name, StartDate: &start, EndDate: &end, StatusOverride: &active}
			if _, err := projects.Update(ctx, fixture.Today, fixture.ProjectID, input); err != nil {
				t.Fatal(err)
			}
			setCheckoutStock(t, database, fixture, 7, 3, 5)
			staffID := insertCheckoutStaff(t, database)
			command := cashCheckoutCommand(t, fixture, staffID, "manual-active", "200.00", false)
			request := models.POSCheckoutRequest{Buyer: command.Buyer, Items: command.Items, Payment: command.Payment}
			catalogue, err := pos.Catalog(ctx, fixture.ProjectID)
			if err != nil || !catalogue.CanCheckout || catalogue.Project.StatusOverride == nil {
				t.Fatalf("catalogue = %+v, error = %v", catalogue, err)
			}
			if _, err := pos.Quote(ctx, fixture.ProjectID, models.POSCartRequest{Items: command.Items}); err != nil {
				t.Fatalf("quote outside planned dates: %v", err)
			}
			order, _, err := pos.Checkout(ctx, fixture.ProjectID, staffID, command.IdempotencyKey, request)
			if err != nil {
				t.Fatalf("checkout outside planned dates: %v", err)
			}
			if err := products.Archive(ctx, fixture.Today, fixture.ProductAID); !errors.Is(err, ErrProductInUse) {
				t.Fatalf("archive manually active product = %v", err)
			}
			assignments := []models.ProjectProductAssignmentInput{
				{ProductID: fixture.ProductAID, ProjectPrice: "100.00"},
				{ProductID: fixture.ProductBID, VariantID: &fixture.VariantOneID, ProjectPrice: "40.00"},
				{ProductID: fixture.ProductBID, VariantID: &fixture.VariantTwoID, ProjectPrice: "40.00"},
			}
			if _, err := projects.ReplaceProducts(ctx, fixture.Today, fixture.ProjectID, assignments); err != nil {
				t.Fatalf("edit manually active products: %v", err)
			}
			promotionInput := models.ProjectPromotionMutation{
				Name: "Manual active promotion", PromotionPrice: mustPromotionAmount(t, "90.00"),
				Items: []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 1}},
			}
			promotion, err := promotions.Create(ctx, fixture.Today, fixture.ProjectID, promotionInput)
			if err != nil {
				t.Fatalf("edit manually active promotions: %v", err)
			}
			if err := promotions.Delete(ctx, fixture.Today, fixture.ProjectID, promotion.PromotionID); err != nil {
				t.Fatal(err)
			}
			for _, status := range []models.ProjectStatus{models.ProjectStatusNotStarted, models.ProjectStatusCompleted} {
				input.StatusOverride = &status
				if _, err := projects.Update(ctx, fixture.Today, fixture.ProjectID, input); err != nil {
					t.Fatalf("status-only update after out-of-period sale: %v", err)
				}
				catalogue, err := pos.Catalog(ctx, fixture.ProjectID)
				if err != nil || catalogue.CanCheckout {
					t.Fatalf("inactive catalogue = %+v, error = %v", catalogue, err)
				}
				if _, err := pos.Quote(ctx, fixture.ProjectID, models.POSCartRequest{Items: command.Items}); !errors.Is(err, usecases.ErrProjectNotActive) {
					t.Fatalf("inactive quote = %v", err)
				}
				if _, _, err := pos.Checkout(ctx, fixture.ProjectID, staffID, string(status), request); !errors.Is(err, usecases.ErrProjectNotActive) {
					t.Fatalf("inactive checkout = %v", err)
				}
			}
			if _, err := projects.ReplaceProducts(ctx, fixture.Today, fixture.ProjectID, assignments); !errors.Is(err, ErrProjectCompleted) {
				t.Fatalf("completed product edit = %v", err)
			}
			if _, err := promotions.Create(ctx, fixture.Today, fixture.ProjectID, promotionInput); !errors.Is(err, ErrProjectCompleted) {
				t.Fatalf("completed promotion edit = %v", err)
			}
			input.StatusOverride = nil
			if _, err := projects.Update(ctx, fixture.Today, fixture.ProjectID, input); err != nil {
				t.Fatalf("clear override after out-of-period sale: %v", err)
			}
			history := usecases.NewOrderService(NewOrderRepository(database), projects)
			listed, err := history.List(ctx, fixture.ProjectID, usecases.OrderListQuery{}, 1, 10)
			if err != nil || listed.Total != 1 || len(listed.Orders) != 1 || listed.Orders[0].OrderID != order.OrderID {
				t.Fatalf("default history = %+v, error = %v", listed, err)
			}
			exported, err := history.Export(ctx, fixture.ProjectID, usecases.OrderListQuery{})
			if err != nil || len(exported) != 1 || exported[0].OrderID != order.OrderID {
				t.Fatalf("default export = %+v, error = %v", exported, err)
			}
		})
	}
}

func TestProjectStatusOverrideCannotReactivateArchivedProducts(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	ctx := context.Background()
	projects := NewProjectRepository(database)
	project, err := projects.Detail(ctx, fixture.Today, fixture.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	completed := models.ProjectStatusCompleted
	input := models.ProjectInput{Name: &project.Name, StartDate: &project.StartDate, EndDate: &project.EndDate, StatusOverride: &completed}
	if _, err := projects.Update(ctx, fixture.Today, fixture.ProjectID, input); err != nil {
		t.Fatal(err)
	}
	if err := NewProductRepository(database).Archive(ctx, fixture.Today, fixture.ProductAID); err != nil {
		t.Fatalf("archive manually completed product: %v", err)
	}
	active, notStarted := models.ProjectStatusActive, models.ProjectStatusNotStarted
	for _, override := range []*models.ProjectStatus{&active, &notStarted, nil} {
		input.StatusOverride = override
		if _, err := projects.Update(ctx, fixture.Today, fixture.ProjectID, input); !errors.Is(err, ErrProjectArchivedProduct) {
			t.Fatalf("reactivate with override %v = %v", override, err)
		}
	}
	unchanged, err := projects.Detail(ctx, fixture.Today, fixture.ProjectID)
	if err != nil || unchanged.Status != completed || unchanged.StatusOverride == nil || *unchanged.StatusOverride != completed {
		t.Fatalf("rejected changes persisted: %+v, error = %v", unchanged, err)
	}
}
