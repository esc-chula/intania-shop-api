package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

const validPromotionMutationJSON = `{"name":"Bundle","promotion_price":"10.00","items":[{"product_id":1,"quantity":1}]}`

func TestPromotionHandlerReadRoutes(t *testing.T) {
	t.Parallel()

	service := &promotionHTTPReadStub{
		listData:   models.ProjectPromotionListData{Promotions: []models.ProjectPromotion{{PromotionID: 7, ProjectID: 3}}},
		detailData: models.ProjectPromotion{PromotionID: 7, ProjectID: 3},
	}
	router := chi.NewRouter()
	NewPromotionHandler(service).Register(router)

	listResponse := servePromotionRequest(router, http.MethodGet, "/projects/3/promotions", "")
	if listResponse.Code != http.StatusOK || service.listCalls != 1 {
		t.Fatalf("list status/calls = %d/%d", listResponse.Code, service.listCalls)
	}
	if !strings.Contains(listResponse.Body.String(), `"promotion_id":7`) {
		t.Fatalf("list body = %s", listResponse.Body.String())
	}

	detailResponse := servePromotionRequest(router, http.MethodGet, "/projects/3/promotions/7", "")
	if detailResponse.Code != http.StatusOK || service.detailCalls != 1 {
		t.Fatalf("detail status/calls = %d/%d", detailResponse.Code, service.detailCalls)
	}
}

func TestPromotionAdminHandlerMutationRoutes(t *testing.T) {
	t.Parallel()

	service := &promotionHTTPAdminStub{}
	router := chi.NewRouter()
	NewPromotionAdminHandler(service).Register(router)

	createResponse := servePromotionRequest(router, http.MethodPost, "/projects/3/promotions", validPromotionMutationJSON)
	if createResponse.Code != http.StatusCreated || service.createCalls != 1 {
		t.Fatalf("create status/calls = %d/%d", createResponse.Code, service.createCalls)
	}

	updateResponse := servePromotionRequest(router, http.MethodPut, "/projects/3/promotions/7", validPromotionMutationJSON)
	if updateResponse.Code != http.StatusOK || service.updateCalls != 1 {
		t.Fatalf("update status/calls = %d/%d", updateResponse.Code, service.updateCalls)
	}

	deleteResponse := servePromotionRequest(router, http.MethodDelete, "/projects/3/promotions/7", "")
	if deleteResponse.Code != http.StatusNoContent || service.deleteCalls != 1 {
		t.Fatalf("delete status/calls = %d/%d", deleteResponse.Code, service.deleteCalls)
	}
}

func TestPromotionAdminHandlerRejectsMalformedMutationBodies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"name":"Bundle","promotion_price":"10.00","items":[{"product_id":1,"quantity":1}],"unexpected":true}`},
		{name: "trailing value", body: validPromotionMutationJSON + validPromotionMutationJSON},
		{name: "numeric price", body: `{"name":"Bundle","promotion_price":10.00,"items":[{"product_id":1,"quantity":1}]}`},
		{name: "malformed price", body: `{"name":"Bundle","promotion_price":"10","items":[{"product_id":1,"quantity":1}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &promotionHTTPAdminStub{}
			router := chi.NewRouter()
			NewPromotionAdminHandler(service).Register(router)

			response := servePromotionRequest(router, http.MethodPost, "/projects/3/promotions", test.body)
			if response.Code != http.StatusBadRequest || service.createCalls != 0 {
				t.Fatalf("status/calls = %d/%d, body = %s", response.Code, service.createCalls, response.Body.String())
			}
			errorResponse := decodePromotionAPIError(t, response)
			if errorResponse.Code != models.ProjectErrorValidation {
				t.Fatalf("error code = %s", errorResponse.Code)
			}
		})
	}
}

func TestPromotionHandlerMapsDomainErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   models.ProjectAPIErrorCode
	}{
		{name: "project not found", err: repositories.ErrProjectNotFound, wantStatus: http.StatusNotFound, wantCode: models.ProjectErrorNotFound},
		{name: "promotion not found", err: repositories.ErrPromotionNotFound, wantStatus: http.StatusNotFound, wantCode: models.ProjectErrorPromotionNotFound},
		{name: "product not sellable", err: repositories.ErrProductNotSellable, wantStatus: http.StatusConflict, wantCode: models.ProjectErrorProductNotSellable},
		{name: "completed project", err: repositories.ErrProjectCompleted, wantStatus: http.StatusConflict, wantCode: models.ProjectErrorCompleted},
		{name: "price exceeds bundle", err: repositories.ErrPromotionPriceExceedsBundle, wantStatus: http.StatusBadRequest, wantCode: models.ProjectErrorValidation},
		{name: "arithmetic overflow", err: models.ErrTHBAmountOverflow, wantStatus: http.StatusBadRequest, wantCode: models.ProjectErrorValidation},
		{name: "validation", err: usecases.PromotionValidationError{Message: "invalid promotion"}, wantStatus: http.StatusBadRequest, wantCode: models.ProjectErrorValidation},
		{name: "unexpected storage error", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantCode: models.ProjectErrorInternal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &promotionHTTPAdminStub{createErr: test.err}
			router := chi.NewRouter()
			NewPromotionAdminHandler(service).Register(router)

			response := servePromotionRequest(router, http.MethodPost, "/projects/3/promotions", validPromotionMutationJSON)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			errorResponse := decodePromotionAPIError(t, response)
			if errorResponse.Code != test.wantCode {
				t.Fatalf("error code = %s, want %s", errorResponse.Code, test.wantCode)
			}
		})
	}
}

func TestPromotionHandlerRejectsMalformedPathIDs(t *testing.T) {
	t.Parallel()

	service := &promotionHTTPReadStub{}
	router := chi.NewRouter()
	NewPromotionHandler(service).Register(router)

	response := servePromotionRequest(router, http.MethodGet, "/projects/not-a-number/promotions", "")
	if response.Code != http.StatusBadRequest || service.listCalls != 0 {
		t.Fatalf("status/calls = %d/%d", response.Code, service.listCalls)
	}
	if errorResponse := decodePromotionAPIError(t, response); errorResponse.Code != models.ProjectErrorValidation {
		t.Fatalf("error code = %s", errorResponse.Code)
	}
}

type promotionHTTPReadStub struct {
	listData    models.ProjectPromotionListData
	detailData  models.ProjectPromotion
	listErr     error
	detailErr   error
	listCalls   int
	detailCalls int
}

func (stub *promotionHTTPReadStub) List(_ context.Context, _ int64) (models.ProjectPromotionListData, error) {
	stub.listCalls++
	return stub.listData, stub.listErr
}

func (stub *promotionHTTPReadStub) Detail(_ context.Context, _, _ int64) (models.ProjectPromotion, error) {
	stub.detailCalls++
	return stub.detailData, stub.detailErr
}

type promotionHTTPAdminStub struct {
	createErr   error
	updateErr   error
	deleteErr   error
	createCalls int
	updateCalls int
	deleteCalls int
}

func (stub *promotionHTTPAdminStub) Create(_ context.Context, _ int64, _ models.ProjectPromotionMutationRequest) (models.ProjectPromotion, error) {
	stub.createCalls++
	return models.ProjectPromotion{PromotionID: 7, ProjectID: 3}, stub.createErr
}

func (stub *promotionHTTPAdminStub) Update(_ context.Context, _, _ int64, _ models.ProjectPromotionMutationRequest) (models.ProjectPromotion, error) {
	stub.updateCalls++
	return models.ProjectPromotion{PromotionID: 7, ProjectID: 3}, stub.updateErr
}

func (stub *promotionHTTPAdminStub) Delete(_ context.Context, _, _ int64) error {
	stub.deleteCalls++
	return stub.deleteErr
}

type promotionAPIError struct {
	Success bool                       `json:"success"`
	Error   string                     `json:"error"`
	Code    models.ProjectAPIErrorCode `json:"code"`
}

func servePromotionRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func decodePromotionAPIError(t *testing.T, response *httptest.ResponseRecorder) promotionAPIError {
	t.Helper()
	var body promotionAPIError
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v; body = %s", err, response.Body.String())
	}
	if body.Success {
		t.Fatalf("error response marked success: %+v", body)
	}
	return body
}

var _ PromotionService = (*promotionHTTPReadStub)(nil)
var _ PromotionAdminService = (*promotionHTTPAdminStub)(nil)
