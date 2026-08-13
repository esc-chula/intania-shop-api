package handlers

import (
	"context"
	"encoding/json"
	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

type StockAdjuster interface {
	Adjust(context.Context, int64, models.AdjustStockRequest, string) (models.StockTransaction, error)
	ProductTransactions(context.Context, int32, int32, int32) (models.StockTransactionListResponse, error)
	VariantTransactions(context.Context, int32, int32, int32) (models.StockTransactionListResponse, error)
	Transactions(context.Context, int32, int32) (models.StockTransactionListResponse, error)
	BulkReduction(context.Context, models.BulkStockReductionRequest) (models.BulkStockReductionResponse, error)
}

type InventoryHandler struct{ inventory StockAdjuster }

func NewInventoryHandler(i StockAdjuster) *InventoryHandler { return &InventoryHandler{inventory: i} }

func (h *InventoryHandler) Register(router chi.Router) {
	router.Post("/products/{id}/stock/adjust", h.adjust)
	router.Get("/products/{id}/stock/transactions", h.productTransactions)
	router.Get("/variants/{id}/stock/transactions", h.variantTransactions)
	router.Get("/stock/transactions", h.transactions)
	router.Post("/stock/bulk-reduction", h.bulkReduction)
}
func (h *InventoryHandler) adjust(w http.ResponseWriter, r *http.Request) {
	identity, ok := middlewares.IdentityFromContext(r.Context())
	if !ok {
		writeError(w, 401, "Authentication required")
		return
	}
	productID, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil {
		writeError(w, 400, "Invalid product ID")
		return
	}
	var input models.AdjustStockRequest
	if e = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); e != nil {
		writeError(w, 400, "Invalid JSON request body")
		return
	}
	out, e := h.inventory.Adjust(r.Context(), productID, input, strconv.FormatInt(identity.UserID, 10))
	if e != nil {
		writeError(w, 400, e.Error())
		return
	}
	writeSuccess(w, 201, out)
}

func inventoryPathID(w http.ResponseWriter, r *http.Request) (int32, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid resource ID")
		return 0, false
	}
	return int32(id), true
}

func inventoryPage(r *http.Request) (int32, int32) {
	page, _ := strconv.ParseInt(r.URL.Query().Get("page"), 10, 32)
	size, _ := strconv.ParseInt(r.URL.Query().Get("page_size"), 10, 32)
	return int32(page), int32(size)
}

func (h *InventoryHandler) productTransactions(w http.ResponseWriter, r *http.Request) {
	id, ok := inventoryPathID(w, r)
	if !ok {
		return
	}
	page, size := inventoryPage(r)
	out, err := h.inventory.ProductTransactions(r.Context(), id, page, size)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, out)
}

func (h *InventoryHandler) variantTransactions(w http.ResponseWriter, r *http.Request) {
	id, ok := inventoryPathID(w, r)
	if !ok {
		return
	}
	page, size := inventoryPage(r)
	out, err := h.inventory.VariantTransactions(r.Context(), id, page, size)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, out)
}

func (h *InventoryHandler) transactions(w http.ResponseWriter, r *http.Request) {
	page, size := inventoryPage(r)
	out, err := h.inventory.Transactions(r.Context(), page, size)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, out)
}

func (h *InventoryHandler) bulkReduction(w http.ResponseWriter, r *http.Request) {
	var input models.BulkStockReductionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}
	out, err := h.inventory.BulkReduction(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, out)
}
