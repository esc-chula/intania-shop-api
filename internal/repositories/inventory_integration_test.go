//go:build integration

package repositories

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
)

func TestInventoryCreatorNameAcrossAdjustAndHistory(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newPromotionIntegrationFixture(t, database)
	staffID := insertCheckoutStaff(t, database)
	setCheckoutStock(t, database, fixture, 10, 10, 10)
	ctx := context.Background()
	var email string
	if err := database.QueryRow(ctx, `SELECT email FROM users WHERE user_id=$1`, staffID).Scan(&email); err != nil {
		t.Fatal(err)
	}
	repository := NewInventoryRepository(database)
	service := usecases.NewInventoryService(repository)
	actorID := fmt.Sprint(staffID)
	tests := []struct {
		name         string
		accountName  *string
		actor        *string
		operatorName *string
		want         *string
	}{
		{name: "account name", accountName: strPtr("  สมชาย  "), actor: &actorID, want: strPtr("สมชาย")},
		{name: "blank name uses email", accountName: strPtr("   "), actor: &actorID, want: &email},
		{name: "null name uses email", actor: &actorID, want: &email},
		{name: "explicit operator", accountName: strPtr("Shared account"), actor: &actorID, operatorName: strPtr("  มายด์  "), want: strPtr("มายด์")},
		{name: "blank operator falls back", accountName: strPtr("Shared account"), actor: &actorID, operatorName: strPtr(" \t "), want: strPtr("Shared account")},
		{name: "unknown account", actor: strPtr("9223372036854775807"), want: strPtr("9223372036854775807")},
		{name: "legacy label", actor: strPtr("local-seed"), want: strPtr("local-seed")},
		{name: "oversized numeric actor", actor: strPtr(strings.Repeat("9", 100)), want: strPtr(strings.Repeat("9", 100))},
		{name: "unattributed legacy transaction"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := database.Exec(ctx, `UPDATE users SET full_name=$1 WHERE user_id=$2`, test.accountName, staffID); err != nil {
				t.Fatal(err)
			}
			for _, variant := range []bool{false, true} {
				productID := fixture.ProductAID
				request := models.AdjustStockRequest{QuantityChange: 1, Reason: "Creator name test"}
				request.OperatorName = test.operatorName
				if test.operatorName != nil {
					request.QuantityChange = -1
				}
				if variant {
					productID = fixture.ProductBID
					request.VariantID = &fixture.VariantOneID
				}
				actor := ""
				if test.actor != nil {
					actor = *test.actor
				}
				created, err := service.Adjust(ctx, productID, request, actor)
				if err != nil {
					t.Fatal(err)
				}
				if test.actor != nil {
					if created.CreatedBy == nil || *created.CreatedBy != *test.want {
						t.Fatalf("adjust creator = %v, want %v", created.CreatedBy, test.want)
					}
				} else if _, err := database.Exec(ctx, `UPDATE stock_transactions SET created_by=NULL WHERE transaction_id=$1`, created.TransactionID); err != nil {
					t.Fatal(err)
				}
				var stored, storedOperator *string
				if err := database.QueryRow(ctx, `SELECT created_by, operator_name FROM stock_transactions WHERE transaction_id=$1`, created.TransactionID).Scan(&stored, &storedOperator); err != nil {
					t.Fatal(err)
				}
				if (stored == nil) != (test.actor == nil) || (stored != nil && *stored != *test.actor) {
					t.Fatalf("stored creator = %v, want original actor %v", stored, test.actor)
				}
				wantOperator := ""
				if test.operatorName != nil {
					wantOperator = strings.TrimSpace(*test.operatorName)
				}
				if (storedOperator == nil) != (wantOperator == "") || (storedOperator != nil && *storedOperator != wantOperator) {
					t.Fatalf("stored operator = %v, want %q", storedOperator, wantOperator)
				}
				if wantOperator != "" {
					if _, err := database.Exec(ctx, `UPDATE users SET full_name='Renamed account' WHERE user_id=$1`, staffID); err != nil {
						t.Fatal(err)
					}
				}
				lists := []struct {
					name string
					read func() (models.StockTransactionListResponse, error)
				}{
					{"global", func() (models.StockTransactionListResponse, error) {
						return repository.ListTransactions(ctx, 0, 100)
					}},
					{"product", func() (models.StockTransactionListResponse, error) {
						return repository.ListProductTransactions(ctx, productID, 0, 100)
					}},
				}
				if variant {
					lists = append(lists, struct {
						name string
						read func() (models.StockTransactionListResponse, error)
					}{"variant", func() (models.StockTransactionListResponse, error) {
						return repository.ListVariantTransactions(ctx, fixture.VariantOneID, 0, 100)
					}})
				}
				for _, list := range lists {
					page, err := list.read()
					if err != nil {
						t.Fatal(err)
					}
					found := false
					for _, transaction := range page.Transactions {
						if transaction.TransactionID != created.TransactionID {
							continue
						}
						found = true
						got := transaction.CreatedBy
						if (got == nil) != (test.want == nil) || (got != nil && *got != *test.want) {
							t.Fatalf("%s creator = %v, want %v", list.name, got, test.want)
						}
					}
					if !found {
						t.Fatalf("%s history omitted transaction %d", list.name, created.TransactionID)
					}
				}
			}
		})
	}
}

func TestInventoryUsesLinkedPOSOperatorSnapshot(t *testing.T) {
	database := openPromotionIntegrationDatabase(t)
	fixture := newOrderIntegrationFixture(t, database)
	ctx := context.Background()
	// These orders predate manual stock operator support. Resolve their
	// existing names without backfilling or copying them into stock history.
	if _, err := database.Exec(ctx, `UPDATE orders SET operator_name='มายด์' WHERE order_id=$1`, fixture.OrderOneID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `UPDATE users SET full_name='Renamed account' WHERE user_id=$1`, fixture.StaffID); err != nil {
		t.Fatal(err)
	}
	wanted := make(map[int64]string)
	var productID, variantID int64
	for _, test := range []struct {
		orderID int64
		name    string
	}{{fixture.OrderOneID, "มายด์"}, {fixture.OrderTwoID, "POS staff"}} {
		order, err := NewOrderRepository(database).Detail(ctx, fixture.ProjectID, test.orderID)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range order.Items {
			wanted[item.InventoryTransactionID] = test.name
			if item.VariantID != nil {
				productID, variantID = item.ProductID, *item.VariantID
			}
			var actor string
			var copiedName *string
			if err := database.QueryRow(ctx, `SELECT created_by, operator_name FROM stock_transactions WHERE transaction_id=$1`, item.InventoryTransactionID).Scan(&actor, &copiedName); err != nil {
				t.Fatal(err)
			}
			if actor != fmt.Sprint(fixture.StaffID) || copiedName != nil {
				t.Fatalf("POS actor/name = %q/%v", actor, copiedName)
			}
		}
	}
	repository := NewInventoryRepository(database)
	// A caller-provided order reference is not proof of an actual POS link.
	referenceType := "ORDER"
	manual, err := repository.Adjust(ctx, productID, models.AdjustStockRequest{
		VariantID: &variantID, QuantityChange: 1, Reason: "Manual reference",
		ReferenceType: &referenceType, ReferenceID: &fixture.OrderOneID,
	}, fmt.Sprint(fixture.StaffID))
	if err != nil || manual.CreatedBy == nil || *manual.CreatedBy != "Renamed account" {
		t.Fatalf("manual adjustment = %+v, error = %v", manual, err)
	}
	wanted[manual.TransactionID] = "Renamed account"
	for _, read := range []func() (models.StockTransactionListResponse, error){
		func() (models.StockTransactionListResponse, error) { return repository.ListTransactions(ctx, 0, 100) },
		func() (models.StockTransactionListResponse, error) {
			return repository.ListProductTransactions(ctx, productID, 0, 100)
		},
		func() (models.StockTransactionListResponse, error) {
			return repository.ListVariantTransactions(ctx, variantID, 0, 100)
		},
	} {
		page, err := read()
		if err != nil || len(page.Transactions) == 0 || page.Total != int64(len(page.Transactions)) {
			t.Fatalf("history = %+v, error = %v", page, err)
		}
		for _, transaction := range page.Transactions {
			want, exists := wanted[transaction.TransactionID]
			if !exists || transaction.CreatedBy == nil || *transaction.CreatedBy != want {
				t.Fatalf("creator for transaction %d = %v, want %q", transaction.TransactionID, transaction.CreatedBy, want)
			}
		}
	}
}
