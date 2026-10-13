package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/repositories"
)

func TestProjectHandlerMapsReferencedProjectProductConflict(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/projects/3/products", strings.NewReader(`{}`))
	response := httptest.NewRecorder()

	writeProjectErrorResponse(response, request, errors.Join(
		repositories.ErrProjectProductPromotion,
		errors.New("product 7 cannot be removed while used by a promotion"),
	), "Unable to replace project products")

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
	var body struct {
		Success bool                       `json:"success"`
		Error   string                     `json:"error"`
		Code    models.ProjectAPIErrorCode `json:"code"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Success || body.Code != models.ProjectErrorConflict {
		t.Fatalf("error response = %+v", body)
	}
	if !strings.Contains(body.Error, "cannot be removed") {
		t.Fatalf("error message = %q", body.Error)
	}
}

func TestProjectHandlerMapsInvalidatingPromotionPriceConflict(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/projects/3/products", strings.NewReader(`{}`))
	response := httptest.NewRecorder()

	writeProjectErrorResponse(response, request, errors.Join(
		repositories.ErrProjectProductPromotionPrice,
		errors.New("promotion 7 price exceeds its recalculated bundle price"),
	), "Unable to replace project products")

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
	var body struct {
		Success bool                       `json:"success"`
		Error   string                     `json:"error"`
		Code    models.ProjectAPIErrorCode `json:"code"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Success || body.Code != models.ProjectErrorConflict {
		t.Fatalf("error response = %+v", body)
	}
	if !strings.Contains(body.Error, "invalidate") {
		t.Fatalf("error message = %q", body.Error)
	}
}

func TestDecodeProjectStatusOverride(t *testing.T) {
	for _, test := range []struct {
		body string
		want models.ProjectStatus
		ok   bool
	}{
		{`{}`, "", true},
		{`{"status_override":null}`, "", true},
		{`{"status_override":"ACTIVE"}`, models.ProjectStatusActive, true},
		{`{"status_override":1}`, "", false},
		{`{"status_override":true}`, "", false},
		{`{"status_overide":"ACTIVE"}`, "", false},
	} {
		t.Run(test.body, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/projects/3", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			input, ok := decodeProjectInput(response, request)
			if ok != test.ok {
				t.Fatalf("decoded = %t, want %t", ok, test.ok)
			}
			if test.want == "" && input.StatusOverride != nil {
				t.Fatalf("override = %v, want nil", input.StatusOverride)
			}
			if test.want != "" && (input.StatusOverride == nil || *input.StatusOverride != test.want) {
				t.Fatalf("override = %v, want %s", input.StatusOverride, test.want)
			}
			if !ok && response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", response.Code)
			}
		})
	}
}
