//go:build integration

package repositories

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/migrations"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPromotionRepositoryCRUDAndHydration(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	repository := NewPromotionRepository(database)

	variantOne := fixture.VariantOneID
	variantTwo := fixture.VariantTwoID
	created, err := repository.Create(context.Background(), fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Starter bundle",
		PromotionPrice: mustPromotionAmount(t, "300.00"),
		Items: []models.ProjectPromotionItemInput{
			{ProductID: fixture.ProductAID, VariantID: nil, Quantity: 2},
			{ProductID: fixture.ProductBID, VariantID: &variantTwo, Quantity: 1},
			{ProductID: fixture.ProductBID, VariantID: &variantOne, Quantity: 3},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if created.ProjectID != fixture.ProjectID || created.Name != "Starter bundle" {
		t.Fatalf("created promotion identity = %+v", created)
	}
	assertPromotionAmounts(t, created, "360.00", "300.00", "60.00")
	assertPromotionItems(t, created.Items, []promotionItemExpectation{
		{productID: fixture.ProductAID, variantID: nil, quantity: 2, name: fixture.ProductAName, unitPrice: "100.00"},
		{productID: fixture.ProductBID, variantID: &variantOne, quantity: 3, name: fixture.ProductBName, size: promotionStringPointer("M"), color: promotionStringPointer("Blue"), unitPrice: "40.00"},
		{productID: fixture.ProductBID, variantID: &variantTwo, quantity: 1, name: fixture.ProductBName, size: promotionStringPointer("L"), color: promotionStringPointer("Red"), unitPrice: "40.00"},
	})

	detail, err := repository.Detail(context.Background(), fixture.ProjectID, created.PromotionID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(detail, created) {
		t.Fatalf("detail = %+v, want created response %+v", detail, created)
	}

	second, err := repository.Create(context.Background(), fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Small bundle",
		PromotionPrice: mustPromotionAmount(t, "50.00"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}

	list, err := repository.List(context.Background(), fixture.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].PromotionID != created.PromotionID || list[1].PromotionID != second.PromotionID {
		t.Fatalf("promotion ordering = %+v", list)
	}

	updated, err := repository.Update(context.Background(), fixture.Today, fixture.ProjectID, created.PromotionID, models.ProjectPromotionMutation{
		Name:           "Reduced bundle",
		PromotionPrice: mustPromotionAmount(t, "120.00"),
		Items: []models.ProjectPromotionItemInput{
			{ProductID: fixture.ProductBID, VariantID: &variantOne, Quantity: 1},
			{ProductID: fixture.ProductAID, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Reduced bundle" {
		t.Fatalf("updated name = %q", updated.Name)
	}
	assertPromotionAmounts(t, updated, "140.00", "120.00", "20.00")
	assertPromotionItems(t, updated.Items, []promotionItemExpectation{
		{productID: fixture.ProductAID, variantID: nil, quantity: 1, name: fixture.ProductAName, unitPrice: "100.00"},
		{productID: fixture.ProductBID, variantID: &variantOne, quantity: 1, name: fixture.ProductBName, size: promotionStringPointer("M"), color: promotionStringPointer("Blue"), unitPrice: "40.00"},
	})

	if err := repository.Delete(context.Background(), fixture.Today, fixture.ProjectID, second.PromotionID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Detail(context.Background(), fixture.ProjectID, second.PromotionID); !errors.Is(err, ErrPromotionNotFound) {
		t.Fatalf("deleted promotion error = %v, want ErrPromotionNotFound", err)
	}
}

func TestPromotionRepositoryUpdateIsAtomicAndResolvesProjectProducts(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	repository := NewPromotionRepository(database)

	variantOne := fixture.VariantOneID
	created, err := repository.Create(context.Background(), fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Atomic bundle",
		PromotionPrice: mustPromotionAmount(t, "80.00"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = repository.Update(context.Background(), fixture.Today, fixture.ProjectID, created.PromotionID, models.ProjectPromotionMutation{
		Name:           "Should not persist",
		PromotionPrice: mustPromotionAmount(t, "1.00"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.OtherProductID, Quantity: 1}},
	})
	if !errors.Is(err, ErrProductNotSellable) {
		t.Fatalf("cross-project update error = %v, want ErrProductNotSellable", err)
	}

	after, err := repository.Detail(context.Background(), fixture.ProjectID, created.PromotionID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, created) {
		t.Fatalf("promotion changed after rejected replacement: before=%+v after=%+v", created, after)
	}

	assignments, err := repository.ResolveProjectProducts(context.Background(), fixture.ProjectID, []models.ProjectPromotionItemInput{
		{ProductID: fixture.ProductAID},
		{ProductID: fixture.ProductBID, VariantID: &variantOne},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 2 || assignments[0].VariantID != nil || assignments[1].VariantID == nil || *assignments[1].VariantID != variantOne {
		t.Fatalf("resolved assignments = %+v", assignments)
	}
	if assignments[0].ProjectProductID != fixture.AssignmentAID || assignments[1].ProjectProductID != fixture.AssignmentBOneID {
		t.Fatalf("resolved assignment IDs = %+v", assignments)
	}

	_, err = repository.ResolveProjectProducts(context.Background(), fixture.ProjectID, []models.ProjectPromotionItemInput{{ProductID: fixture.OtherProductID, Quantity: 1}})
	if !errors.Is(err, ErrProductNotSellable) {
		t.Fatalf("unassigned product error = %v, want ErrProductNotSellable", err)
	}
}

func TestPromotionRepositoryRecomputesBundlePriceAfterProjectPriceChange(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	repository := NewPromotionRepository(database)

	created, err := repository.Create(context.Background(), fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Dynamic price bundle",
		PromotionPrice: mustPromotionAmount(t, "150.00"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPromotionAmounts(t, created, "200.00", "150.00", "50.00")

	if _, err := database.Exec(context.Background(), `
		UPDATE project_products
		SET project_price = $2::numeric
		WHERE project_product_id = $1`, fixture.AssignmentAID, "125.00"); err != nil {
		t.Fatal(err)
	}

	updated, err := repository.Detail(context.Background(), fixture.ProjectID, created.PromotionID)
	if err != nil {
		t.Fatal(err)
	}
	assertPromotionAmounts(t, updated, "250.00", "150.00", "100.00")
	if updated.Items[0].UnitPrice.String() != "125.00" {
		t.Fatalf("hydrated unit price = %s, want 125.00", updated.Items[0].UnitPrice.String())
	}
}

func TestPromotionRepositoryRejectsCompletedProjectMutationsButAllowsReads(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	repository := NewPromotionRepository(database)

	created, err := repository.Create(context.Background(), fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Lifecycle bundle",
		PromotionPrice: mustPromotionAmount(t, "50.00"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := database.Exec(context.Background(), `
		UPDATE projects
		SET end_date = $2::date
		WHERE project_id = $1`, fixture.ProjectID, fixture.Today.Time.AddDate(0, 0, -1).Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}

	if _, err := repository.Detail(context.Background(), fixture.ProjectID, created.PromotionID); err != nil {
		t.Fatalf("completed-project detail error = %v", err)
	}
	if _, err := repository.List(context.Background(), fixture.ProjectID); err != nil {
		t.Fatalf("completed-project list error = %v", err)
	}

	mutation := models.ProjectPromotionMutation{
		Name:           "Rejected mutation",
		PromotionPrice: mustPromotionAmount(t, "50.00"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 1}},
	}
	if _, err := repository.Update(context.Background(), fixture.Today, fixture.ProjectID, created.PromotionID, mutation); !errors.Is(err, ErrProjectCompleted) {
		t.Fatalf("completed update error = %v, want ErrProjectCompleted", err)
	}
	if err := repository.Delete(context.Background(), fixture.Today, fixture.ProjectID, created.PromotionID); !errors.Is(err, ErrProjectCompleted) {
		t.Fatalf("completed delete error = %v, want ErrProjectCompleted", err)
	}
	if _, err := repository.Create(context.Background(), fixture.Today, fixture.ProjectID, mutation); !errors.Is(err, ErrProjectCompleted) {
		t.Fatalf("completed create error = %v, want ErrProjectCompleted", err)
	}
}

func TestPromotionRepositoryEnforcesPromotionItemRestrictAndCascade(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	repository := NewPromotionRepository(database)

	created, err := repository.Create(context.Background(), fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Foreign key bundle",
		PromotionPrice: mustPromotionAmount(t, "50.00"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := database.Exec(context.Background(), `DELETE FROM project_products WHERE project_product_id = $1`, fixture.AssignmentAID); err == nil {
		t.Fatal("deleting a referenced project product succeeded")
	} else {
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || (databaseError.Code != "23503" && databaseError.Code != "23001") {
			t.Fatalf("delete referenced project product error = %v, want foreign-key/restrict violation", err)
		}
	}

	if err := repository.Delete(context.Background(), fixture.Today, fixture.ProjectID, created.PromotionID); err != nil {
		t.Fatal(err)
	}
	var itemCount int
	if err := database.QueryRow(context.Background(), `SELECT count(*) FROM promotion_items WHERE promotion_id = $1`, created.PromotionID).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if itemCount != 0 {
		t.Fatalf("promotion item count after promotion delete = %d, want 0", itemCount)
	}

	if _, err := database.Exec(context.Background(), `DELETE FROM project_products WHERE project_product_id = $1`, fixture.AssignmentAID); err != nil {
		t.Fatalf("unreferenced project product delete error = %v", err)
	}
}

func TestPromotionRepositoryMapsMissingProjectsPromotionsAndExcessivePrice(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	repository := NewPromotionRepository(database)
	missingID := int64(9223372036854770000)

	if _, err := repository.List(context.Background(), missingID); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("missing project list error = %v, want ErrProjectNotFound", err)
	}
	if _, err := repository.Detail(context.Background(), missingID, 1); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("missing project detail error = %v, want ErrProjectNotFound", err)
	}
	if _, err := repository.Detail(context.Background(), fixture.ProjectID, missingID); !errors.Is(err, ErrPromotionNotFound) {
		t.Fatalf("missing promotion detail error = %v, want ErrPromotionNotFound", err)
	}
	created, err := repository.Create(context.Background(), fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Scoped promotion",
		PromotionPrice: mustPromotionAmount(t, "50.00"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Detail(context.Background(), fixture.OtherProjectID, created.PromotionID); !errors.Is(err, ErrPromotionNotFound) {
		t.Fatalf("wrong-project promotion detail error = %v, want ErrPromotionNotFound", err)
	}

	_, err = repository.Create(context.Background(), fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Too expensive",
		PromotionPrice: mustPromotionAmount(t, "100.01"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 1}},
	})
	if !errors.Is(err, ErrPromotionPriceExceedsBundle) {
		t.Fatalf("excessive price error = %v, want ErrPromotionPriceExceedsBundle", err)
	}
	list, err := repository.List(context.Background(), fixture.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].PromotionID != created.PromotionID {
		t.Fatalf("promotion list after rejected create = %+v, want only promotion %d", list, created.PromotionID)
	}
}

func TestProjectProductReplacementPreservesPromotionReferences(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	promotionRepository := NewPromotionRepository(database)
	projectRepository := NewProjectRepository(database)

	created, err := promotionRepository.Create(context.Background(), fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Referenced assignment bundle",
		PromotionPrice: mustPromotionAmount(t, "150.00"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}

	before, err := promotionRepository.ResolveProjectProducts(context.Background(), fixture.ProjectID,
		[]models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].ProjectProductID != fixture.AssignmentAID {
		t.Fatalf("referenced assignment before replacement = %+v", before)
	}

	variantOne := fixture.VariantOneID
	variantTwo := fixture.VariantTwoID
	if _, err := projectRepository.ReplaceProducts(context.Background(), fixture.Today, fixture.ProjectID, []models.ProjectProductAssignmentInput{
		{ProductID: fixture.ProductAID, ProjectPrice: "125.00"},
		{ProductID: fixture.ProductBID, VariantID: &variantOne, ProjectPrice: "40.00"},
		{ProductID: fixture.ProductBID, VariantID: &variantTwo, ProjectPrice: "40.00"},
	}); err != nil {
		t.Fatalf("replacement retaining referenced assignment: %v", err)
	}

	after, err := promotionRepository.ResolveProjectProducts(context.Background(), fixture.ProjectID,
		[]models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].ProjectProductID != before[0].ProjectProductID {
		t.Fatalf("referenced assignment identity changed: before=%+v after=%+v", before, after)
	}
	if after[0].ProjectPrice.String() != "125.00" {
		t.Fatalf("repriced assignment = %s, want 125.00", after[0].ProjectPrice.String())
	}

	hydrated, err := promotionRepository.Detail(context.Background(), fixture.ProjectID, created.PromotionID)
	if err != nil {
		t.Fatal(err)
	}
	assertPromotionAmounts(t, hydrated, "250.00", "150.00", "100.00")
	if hydrated.Items[0].UnitPrice.String() != "125.00" {
		t.Fatalf("hydrated repriced unit = %s, want 125.00", hydrated.Items[0].UnitPrice.String())
	}

	_, err = projectRepository.ReplaceProducts(context.Background(), fixture.Today, fixture.ProjectID, []models.ProjectProductAssignmentInput{
		{ProductID: fixture.ProductBID, VariantID: &variantOne, ProjectPrice: "40.00"},
		{ProductID: fixture.ProductBID, VariantID: &variantTwo, ProjectPrice: "40.00"},
	})
	if !errors.Is(err, ErrProjectProductPromotion) {
		t.Fatalf("removing referenced assignment error = %v, want ErrProjectProductPromotion", err)
	}

	unchanged, err := promotionRepository.Detail(context.Background(), fixture.ProjectID, created.PromotionID)
	if err != nil {
		t.Fatal(err)
	}
	assertPromotionAmounts(t, unchanged, "250.00", "150.00", "100.00")
	if _, err := promotionRepository.ResolveProjectProducts(context.Background(), fixture.ProjectID,
		[]models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID}}); err != nil {
		t.Fatalf("referenced assignment disappeared after rejected replacement: %v", err)
	}
}

func TestProjectProductReplacementRejectsInvalidatingPromotionReprice(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	promotionRepository := NewPromotionRepository(database)
	projectRepository := NewProjectRepository(database)

	created, err := promotionRepository.Create(context.Background(), fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Protected price bundle",
		PromotionPrice: mustPromotionAmount(t, "150.00"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPromotionAmounts(t, created, "200.00", "150.00", "50.00")

	variantOne := fixture.VariantOneID
	variantTwo := fixture.VariantTwoID
	_, err = projectRepository.ReplaceProducts(context.Background(), fixture.Today, fixture.ProjectID, []models.ProjectProductAssignmentInput{
		{ProductID: fixture.ProductAID, ProjectPrice: "70.00"},
		{ProductID: fixture.ProductBID, VariantID: &variantOne, ProjectPrice: "40.00"},
		{ProductID: fixture.ProductBID, VariantID: &variantTwo, ProjectPrice: "40.00"},
	})
	if !errors.Is(err, ErrProjectProductPromotionPrice) {
		t.Fatalf("invalidating reprice error = %v, want ErrProjectProductPromotionPrice", err)
	}

	var projectPrice string
	if err := database.QueryRow(context.Background(), `
		SELECT project_price::text
		FROM project_products
		WHERE project_product_id = $1`, fixture.AssignmentAID).Scan(&projectPrice); err != nil {
		t.Fatal(err)
	}
	if projectPrice != "100.00" {
		t.Fatalf("project price after rejected reprice = %s, want 100.00", projectPrice)
	}

	unchanged, err := promotionRepository.Detail(context.Background(), fixture.ProjectID, created.PromotionID)
	if err != nil {
		t.Fatal(err)
	}
	assertPromotionAmounts(t, unchanged, "200.00", "150.00", "50.00")
}

func TestProjectDeleteCascadesPromotionsAndItems(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	promotionRepository := NewPromotionRepository(database)
	projectRepository := NewProjectRepository(database)

	created, err := promotionRepository.Create(context.Background(), fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Project deletion bundle",
		PromotionPrice: mustPromotionAmount(t, "50.00"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := projectRepository.Delete(context.Background(), fixture.ProjectID); err != nil {
		t.Fatalf("delete project with promotion and no orders: %v", err)
	}

	for name, query := range map[string]string{
		"project":     `SELECT count(*) FROM projects WHERE project_id = $1`,
		"promotion":   `SELECT count(*) FROM promotions WHERE promotion_id = $1`,
		"item":        `SELECT count(*) FROM promotion_items WHERE promotion_id = $1`,
		"assignments": `SELECT count(*) FROM project_products WHERE project_id = $1`,
	} {
		var count int
		argument := fixture.ProjectID
		if name == "promotion" || name == "item" {
			argument = created.PromotionID
		}
		if err := database.QueryRow(context.Background(), query, argument).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if count != 0 {
			t.Fatalf("%s count after project deletion = %d, want 0", name, count)
		}
	}
}

func TestProjectDeleteWithOrdersRollsBackPromotionCleanup(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	promotionRepository := NewPromotionRepository(database)
	projectRepository := NewProjectRepository(database)

	created, err := promotionRepository.Create(context.Background(), fixture.Today, fixture.ProjectID, models.ProjectPromotionMutation{
		Name:           "Protected project bundle",
		PromotionPrice: mustPromotionAmount(t, "50.00"),
		Items:          []models.ProjectPromotionItemInput{{ProductID: fixture.ProductAID, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(context.Background(), `INSERT INTO orders (project_id) VALUES ($1)`, fixture.ProjectID); err != nil {
		t.Fatal(err)
	}

	if err := projectRepository.Delete(context.Background(), fixture.ProjectID); !errors.Is(err, ErrProjectHasOrders) {
		t.Fatalf("delete project with orders error = %v, want ErrProjectHasOrders", err)
	}

	for name, query := range map[string]string{
		"project":   `SELECT count(*) FROM projects WHERE project_id = $1`,
		"promotion": `SELECT count(*) FROM promotions WHERE promotion_id = $1`,
		"item":      `SELECT count(*) FROM promotion_items WHERE promotion_id = $1`,
	} {
		var count int
		argument := fixture.ProjectID
		if name == "promotion" || name == "item" {
			argument = created.PromotionID
		}
		if err := database.QueryRow(context.Background(), query, argument).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if count != 1 {
			t.Fatalf("%s count after rejected deletion = %d, want 1", name, count)
		}
	}
}

type promotionIntegrationFixture struct {
	Today             models.Date
	ProjectID         int64
	OtherProjectID    int64
	ProductAID        int64
	ProductBID        int64
	OtherProductID    int64
	ProductAName      string
	ProductBName      string
	VariantOneID      int64
	VariantTwoID      int64
	AssignmentAID     int64
	AssignmentBOneID  int64
	AssignmentBTwoID  int64
	OtherAssignmentID int64
}

func openPromotionIntegrationDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := migrations.Apply(ctx, dsn, logger); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	database, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL pool: %v", err)
	}
	if err := database.Ping(ctx); err != nil {
		database.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}
	t.Cleanup(database.Close)
	return database
}

func newPromotionIntegrationFixture(t *testing.T, database *pgxpool.Pool) promotionIntegrationFixture {
	t.Helper()
	today := models.TodayInBangkok(time.Now())
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	startDate := today.Time.AddDate(0, 0, -1).Format("2006-01-02")
	endDate := today.Time.AddDate(0, 0, 1).Format("2006-01-02")

	projectID := insertPromotionProject(t, database, "Promotion test project "+suffix, startDate, endDate)
	otherProjectID := insertPromotionProject(t, database, "Other promotion project "+suffix, startDate, endDate)

	productAName := "Promotion test product A " + suffix
	productBName := "Promotion test product B " + suffix
	productAID := insertPromotionProduct(t, database, productAName, "100.00")
	productBID := insertPromotionProduct(t, database, productBName, "40.00")
	otherProductID := insertPromotionProduct(t, database, "Other promotion product "+suffix, "70.00")
	variantOneID := insertPromotionVariant(t, database, productBID, "M", "Blue", "40.00")
	variantTwoID := insertPromotionVariant(t, database, productBID, "L", "Red", "45.00")

	assignmentAID := insertPromotionProjectProduct(t, database, projectID, productAID, nil, "100.00")
	assignmentBOneID := insertPromotionProjectProduct(t, database, projectID, productBID, &variantOneID, "40.00")
	assignmentBTwoID := insertPromotionProjectProduct(t, database, projectID, productBID, &variantTwoID, "40.00")
	otherAssignmentID := insertPromotionProjectProduct(t, database, otherProjectID, otherProductID, nil, "70.00")

	fixture := promotionIntegrationFixture{
		Today:             today,
		ProjectID:         projectID,
		OtherProjectID:    otherProjectID,
		ProductAID:        productAID,
		ProductBID:        productBID,
		OtherProductID:    otherProductID,
		ProductAName:      productAName,
		ProductBName:      productBName,
		VariantOneID:      variantOneID,
		VariantTwoID:      variantTwoID,
		AssignmentAID:     assignmentAID,
		AssignmentBOneID:  assignmentBOneID,
		AssignmentBTwoID:  assignmentBTwoID,
		OtherAssignmentID: otherAssignmentID,
	}
	t.Cleanup(func() { cleanupPromotionIntegrationFixture(t, database, fixture) })
	return fixture
}

func cleanupPromotionIntegrationFixture(t *testing.T, database *pgxpool.Pool, fixture promotionIntegrationFixture) {
	t.Helper()
	ctx := context.Background()
	for _, projectID := range []int64{fixture.ProjectID, fixture.OtherProjectID} {
		if _, err := database.Exec(ctx, `DELETE FROM orders WHERE project_id = $1`, projectID); err != nil {
			t.Errorf("cleanup orders for project %d: %v", projectID, err)
		}
		if _, err := database.Exec(ctx, `DELETE FROM promotions WHERE project_id = $1`, projectID); err != nil {
			t.Errorf("cleanup promotions for project %d: %v", projectID, err)
		}
		if _, err := database.Exec(ctx, `DELETE FROM project_products WHERE project_id = $1`, projectID); err != nil {
			t.Errorf("cleanup project products for project %d: %v", projectID, err)
		}
		if _, err := database.Exec(ctx, `DELETE FROM projects WHERE project_id = $1`, projectID); err != nil {
			t.Errorf("cleanup project %d: %v", projectID, err)
		}
	}
	for _, productID := range []int64{fixture.ProductAID, fixture.ProductBID, fixture.OtherProductID} {
		if _, err := database.Exec(ctx, `DELETE FROM products WHERE id = $1`, productID); err != nil {
			t.Errorf("cleanup product %d: %v", productID, err)
		}
	}
}

func insertPromotionProject(t *testing.T, database *pgxpool.Pool, name, startDate, endDate string) int64 {
	t.Helper()
	var projectID int64
	if err := database.QueryRow(context.Background(), `
		INSERT INTO projects (name, start_date, end_date)
		VALUES ($1, $2::date, $3::date)
		RETURNING project_id`, name, startDate, endDate).Scan(&projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	return projectID
}

func insertPromotionProduct(t *testing.T, database *pgxpool.Pool, name, price string) int64 {
	t.Helper()
	var productID int64
	if err := database.QueryRow(context.Background(), `
		INSERT INTO products (name, price, status)
		VALUES ($1, $2::numeric, 'IN_STOCK')
		RETURNING id`, name, price).Scan(&productID); err != nil {
		t.Fatalf("insert product: %v", err)
	}
	return productID
}

func insertPromotionVariant(t *testing.T, database *pgxpool.Pool, productID int64, size, color, price string) int64 {
	t.Helper()
	var variantID int64
	if err := database.QueryRow(context.Background(), `
		INSERT INTO variants (product_id, size, color, price)
		VALUES ($1, $2, $3, $4::numeric)
		RETURNING variant_id`, productID, size, color, price).Scan(&variantID); err != nil {
		t.Fatalf("insert variant: %v", err)
	}
	return variantID
}

func insertPromotionProjectProduct(t *testing.T, database *pgxpool.Pool, projectID, productID int64, variantID *int64, price string) int64 {
	t.Helper()
	var projectProductID int64
	if err := database.QueryRow(context.Background(), `
		INSERT INTO project_products (project_id, product_id, variant_id, project_price)
		VALUES ($1, $2, $3, $4::numeric)
		RETURNING project_product_id`, projectID, productID, variantID, price).Scan(&projectProductID); err != nil {
		t.Fatalf("insert project product: %v", err)
	}
	return projectProductID
}

type promotionItemExpectation struct {
	productID int64
	variantID *int64
	quantity  int32
	name      string
	size      *string
	color     *string
	unitPrice string
}

func assertPromotionItems(t *testing.T, items []models.ProjectPromotionItem, expectations []promotionItemExpectation) {
	t.Helper()
	if len(items) != len(expectations) {
		t.Fatalf("item count = %d, want %d; items = %+v", len(items), len(expectations), items)
	}
	for index, expectation := range expectations {
		item := items[index]
		if item.ProductID != expectation.productID || item.Quantity != expectation.quantity || item.ProductName != expectation.name || item.UnitPrice.String() != expectation.unitPrice {
			t.Fatalf("item %d = %+v, want product=%d quantity=%d name=%q unit_price=%s", index, item, expectation.productID, expectation.quantity, expectation.name, expectation.unitPrice)
		}
		if (item.VariantID == nil) != (expectation.variantID == nil) {
			t.Fatalf("item %d variant = %v, want %v", index, item.VariantID, expectation.variantID)
		}
		if item.VariantID != nil && *item.VariantID != *expectation.variantID {
			t.Fatalf("item %d variant = %d, want %d", index, *item.VariantID, *expectation.variantID)
		}
		if (item.Size == nil) != (expectation.size == nil) || (item.Size != nil && *item.Size != *expectation.size) {
			t.Fatalf("item %d size = %v, want %v", index, item.Size, expectation.size)
		}
		if (item.Color == nil) != (expectation.color == nil) || (item.Color != nil && *item.Color != *expectation.color) {
			t.Fatalf("item %d color = %v, want %v", index, item.Color, expectation.color)
		}
	}
}

func assertPromotionAmounts(t *testing.T, promotion models.ProjectPromotion, original, promotionPrice, discount string) {
	t.Helper()
	if promotion.OriginalBundlePrice.String() != original || promotion.PromotionPrice.String() != promotionPrice || promotion.Discount.String() != discount {
		t.Fatalf("promotion amounts = original %s, promotion %s, discount %s; want %s, %s, %s", promotion.OriginalBundlePrice.String(), promotion.PromotionPrice.String(), promotion.Discount.String(), original, promotionPrice, discount)
	}
}

func mustPromotionAmount(t *testing.T, value string) models.THBAmount {
	t.Helper()
	amount, err := models.ParseTHBAmount(value)
	if err != nil {
		t.Fatalf("parse test amount %q: %v", value, err)
	}
	return amount
}

func promotionStringPointer(value string) *string { return &value }
