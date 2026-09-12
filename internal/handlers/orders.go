package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
	"github.com/xuri/excelize/v2"
)

// OrderService is the application surface needed by the order history HTTP
// adapter.
type OrderService interface {
	List(context.Context, int64, usecases.OrderListQuery, int32, int32) (models.OrderListResponse, error)
	Export(context.Context, int64, usecases.OrderListQuery) ([]models.Order, error)
	Detail(context.Context, int64, int64) (models.Order, error)
}

// OrderHandler serves the project-scoped POS order history endpoints.
type OrderHandler struct {
	orders OrderService
}

// NewOrderHandler constructs an order HTTP adapter.
func NewOrderHandler(orders OrderService) *OrderHandler {
	return &OrderHandler{orders: orders}
}

// Register mounts order routes on an already-authenticated router.
func (handler *OrderHandler) Register(router chi.Router) {
	router.Get("/projects/{project_id}/orders", handler.list)
	router.Get("/projects/{project_id}/orders/export", handler.export)
	router.Get("/projects/{project_id}/orders/{order_id}", handler.detail)
}

func (handler *OrderHandler) list(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}
	data, err := handler.orders.List(request.Context(), projectID, orderListQueryFromRequest(request),
		queryInt(request, "page", 1), queryInt(request, "page_size", 10))
	if err != nil {
		writeProjectErrorResponse(writer, request, err, "Unable to list orders")
		return
	}
	writeSuccess(writer, http.StatusOK, data)
}

func (handler *OrderHandler) detail(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}
	orderID, ok := orderPathID(writer, request)
	if !ok {
		return
	}
	order, err := handler.orders.Detail(request.Context(), projectID, orderID)
	if err != nil {
		writeProjectErrorResponse(writer, request, err, "Unable to get order")
		return
	}
	writeSuccess(writer, http.StatusOK, order)
}

func (handler *OrderHandler) export(writer http.ResponseWriter, request *http.Request) {
	projectID, ok := projectPathID(writer, request)
	if !ok {
		return
	}
	orders, err := handler.orders.Export(request.Context(), projectID, orderListQueryFromRequest(request))
	if err != nil {
		writeProjectErrorResponse(writer, request, err, "Unable to export orders")
		return
	}

	workbook, err := buildOrderExportWorkbook(orders)
	if err != nil {
		writeProjectError(writer, request, http.StatusInternalServerError, models.ProjectErrorInternal, "Unable to export orders")
		return
	}

	filename := fmt.Sprintf("orders_%s.xlsx", time.Now().Format("20060102T150405"))
	writer.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, url.PathEscape(filename)))
	writer.WriteHeader(http.StatusOK)
	_ = workbook.Write(writer)
}

func orderPathID(writer http.ResponseWriter, request *http.Request) (int64, bool) {
	orderID, err := strconv.ParseInt(request.PathValue("order_id"), 10, 64)
	if err != nil {
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorValidation, "Invalid order ID")
		return 0, false
	}
	return orderID, true
}

func orderListQueryFromRequest(request *http.Request) usecases.OrderListQuery {
	query := request.URL.Query()
	return usecases.OrderListQuery{
		OrderNumber:   query.Get("order_number"),
		PaymentMethod: query.Get("payment_method"),
		StaffID:       int64(queryInt(request, "staff_id", 0)),
		CreatedFrom:   query.Get("created_from"),
		CreatedTo:     query.Get("created_to"),
		SortBy:        query.Get("sort_by"),
		SortOrder:     query.Get("sort_order"),
	}
}

// orderExportColumns are the export sheet columns, in order. One row is
// written per order item, so order-level fields repeat across the item rows
// they contain.
var orderExportColumns = []string{
	"Order Number", "Created At", "Payment Method", "Staff Name", "Staff Email",
	"Buyer Gender", "Buyer Age", "Buyer Student/Alumni Year",
	"Product Name", "Size", "Color", "Quantity", "Unit Price", "Line Total",
	"Promotion Name", "Discount", "Subtotal", "Net Total",
}

func buildOrderExportWorkbook(orders []models.Order) (*excelize.File, error) {
	workbook := excelize.NewFile()
	const sheet = "Orders"
	if index, err := workbook.NewSheet(sheet); err == nil {
		workbook.SetActiveSheet(index)
	}
	if err := workbook.DeleteSheet("Sheet1"); err != nil {
		return nil, fmt.Errorf("prepare export workbook: %w", err)
	}

	for column, header := range orderExportColumns {
		cell, err := excelize.CoordinatesToCellName(column+1, 1)
		if err != nil {
			return nil, fmt.Errorf("build export header: %w", err)
		}
		if err := workbook.SetCellValue(sheet, cell, header); err != nil {
			return nil, fmt.Errorf("write export header: %w", err)
		}
	}

	row := 2
	for _, order := range orders {
		promotionName, discount := "", "0.00"
		if order.AppliedPromotion != nil {
			promotionName = order.AppliedPromotion.Name
			discount = order.AppliedPromotion.Discount.String()
		}

		for _, item := range order.Items {
			values := []any{
				order.OrderNumber,
				order.CreatedAt.Format(time.RFC3339),
				string(order.Payment.Method),
				order.Staff.FullName,
				order.Staff.Email,
				string(order.Buyer.Gender),
				optionalInt32(order.Buyer.Age),
				optionalString(order.Buyer.StudentAlumniYear),
				item.ProductName,
				optionalString(item.Size),
				optionalString(item.Color),
				item.Quantity,
				item.UnitPrice.String(),
				item.LineTotal.String(),
				promotionName,
				discount,
				order.Subtotal.String(),
				order.NetTotal.String(),
			}
			for column, value := range values {
				cell, err := excelize.CoordinatesToCellName(column+1, row)
				if err != nil {
					return nil, fmt.Errorf("build export row: %w", err)
				}
				if err := workbook.SetCellValue(sheet, cell, value); err != nil {
					return nil, fmt.Errorf("write export row: %w", err)
				}
			}
			row++
		}
	}

	return workbook, nil
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalInt32(value *int32) any {
	if value == nil {
		return ""
	}
	return *value
}
