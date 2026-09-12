package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

// pngBytes is a minimal body whose signature makes content sniffing report
// image/png.
var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)

func TestPaymentSlipHandlerUploadReturnsATrustedObjectKey(t *testing.T) {
	service := &paymentSlipServiceStub{slip: models.PaymentSlip{
		ObjectKey:   "payment-slips/1757000000-slip.png",
		URL:         "https://storage.googleapis.com/bucket/payment-slips/1757000000-slip.png",
		ContentType: "image/png",
		Size:        int64(len(pngBytes)),
	}}
	router := paymentSlipRouter(service)

	body, contentType := paymentSlipMultipart(t, "file", "slip.png", pngBytes)
	response := servePaymentSlipRequest(router, body, contentType, true)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}

	if service.calls != 1 || service.upload.UploadedBy != 42 || service.upload.Filename != "slip.png" {
		t.Fatalf("upload = %+v over %d calls", service.upload, service.calls)
	}
	// The content type is sniffed from the bytes, never taken from the part
	// header the client supplied.
	if service.upload.ContentType != "image/png" || service.upload.Size != int64(len(pngBytes)) {
		t.Fatalf("detected upload = %+v", service.upload)
	}
	if service.content != string(pngBytes) {
		t.Fatalf("uploaded %d bytes, want %d", len(service.content), len(pngBytes))
	}

	var envelope struct {
		Success bool               `json:"success"`
		Data    models.PaymentSlip `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Success || envelope.Data.ObjectKey != service.slip.ObjectKey || envelope.Data.Size != service.slip.Size {
		t.Fatalf("slip response = %+v", envelope)
	}
}

func TestPaymentSlipHandlerUploadSniffsTheContentTypeItForwards(t *testing.T) {
	service := &paymentSlipServiceStub{}
	router := paymentSlipRouter(service)

	// A text file renamed to .png is reported as text, so the use case can
	// reject it.
	body, contentType := paymentSlipMultipart(t, "file", "slip.png", []byte("not really an image"))
	servePaymentSlipRequest(router, body, contentType, true)

	if !strings.HasPrefix(service.upload.ContentType, "text/plain") {
		t.Fatalf("detected content type = %q", service.upload.ContentType)
	}
}

func TestPaymentSlipHandlerUploadRejectsMalformedRequests(t *testing.T) {
	tests := []struct {
		name  string
		field string
		files int
	}{
		{name: "wrong field name", field: "image", files: 1},
		{name: "no file at all", field: "file", files: 0},
		{name: "more than one file", field: "file", files: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &paymentSlipServiceStub{}
			router := paymentSlipRouter(service)

			buffer := &bytes.Buffer{}
			writer := multipart.NewWriter(buffer)
			for index := 0; index < test.files; index++ {
				part, err := writer.CreateFormFile(test.field, "slip.png")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := part.Write(pngBytes); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}

			response := servePaymentSlipRequest(router, buffer.Bytes(), writer.FormDataContentType(), true)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
			}
			if apiError := decodePOSAPIError(t, response); apiError.Code != models.ProjectErrorInvalidPaymentSlip {
				t.Fatalf("error code = %s", apiError.Code)
			}
			if service.calls != 0 {
				t.Fatal("a malformed upload reached the service")
			}
		})
	}
}

func TestPaymentSlipHandlerUploadSeparatesOversizedFromMalformedBodies(t *testing.T) {
	// A body past the multipart envelope limit is the only 413 the parse path
	// can produce. Everything else it rejects is a malformed request.
	t.Run("a body larger than the limit", func(t *testing.T) {
		service := &paymentSlipServiceStub{}
		router := paymentSlipRouter(service)

		oversized := append(pngBytes, bytes.Repeat([]byte{0}, int(paymentSlipFormLimit))...)
		body, contentType := paymentSlipMultipart(t, "file", "slip.png", oversized)
		response := servePaymentSlipRequest(router, body, contentType, true)
		if response.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
		}
		if apiError := decodePOSAPIError(t, response); apiError.Code != models.ProjectErrorInvalidPaymentSlip {
			t.Fatalf("error code = %s", apiError.Code)
		}
		if service.calls != 0 {
			t.Fatal("an oversized upload reached the service")
		}
	})

	malformed := []struct {
		name        string
		body        []byte
		contentType string
	}{
		{name: "a JSON body", body: []byte(`{"a":1}`), contentType: "application/json"},
		{name: "no content type", body: []byte(`{"a":1}`), contentType: ""},
		{name: "a truncated multipart body", body: []byte("--boundary\r\nContent-Dis"),
			contentType: "multipart/form-data; boundary=boundary"},
	}
	for _, test := range malformed {
		t.Run(test.name, func(t *testing.T) {
			service := &paymentSlipServiceStub{}
			router := paymentSlipRouter(service)

			response := servePaymentSlipRequest(router, test.body, test.contentType, true)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
			}
			if apiError := decodePOSAPIError(t, response); apiError.Code != models.ProjectErrorInvalidPaymentSlip {
				t.Fatalf("error code = %s", apiError.Code)
			}
			if service.calls != 0 {
				t.Fatal("a malformed upload reached the service")
			}
		})
	}
}

func TestPaymentSlipHandlerUploadRejectsASecondFilePart(t *testing.T) {
	service := &paymentSlipServiceStub{}
	router := paymentSlipRouter(service)

	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	for _, field := range []string{"file", "extra"} {
		part, err := writer.CreateFormFile(field, field+".png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(pngBytes); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	response := servePaymentSlipRequest(router, buffer.Bytes(), writer.FormDataContentType(), true)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if service.calls != 0 {
		t.Fatal("an upload carrying a second file reached the service")
	}
}

func TestPaymentSlipHandlerUploadMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "unsupported type", err: usecases.ErrPaymentSlipUnsupportedType, wantStatus: http.StatusBadRequest},
		{name: "empty file", err: usecases.ErrPaymentSlipEmpty, wantStatus: http.StatusBadRequest},
		{name: "too large", err: usecases.ErrPaymentSlipTooLarge, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "storage failure", err: errors.New("bucket unavailable"), wantStatus: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := paymentSlipRouter(&paymentSlipServiceStub{err: test.err})

			body, contentType := paymentSlipMultipart(t, "file", "slip.png", pngBytes)
			response := servePaymentSlipRequest(router, body, contentType, true)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, test.wantStatus, response.Body.String())
			}

			apiError := decodePOSAPIError(t, response)
			wantCode := models.ProjectErrorInvalidPaymentSlip
			if test.wantStatus == http.StatusInternalServerError {
				wantCode = models.ProjectErrorInternal
			}
			if apiError.Code != wantCode {
				t.Fatalf("error code = %s, want %s", apiError.Code, wantCode)
			}
		})
	}
}

func TestPaymentSlipHandlerUploadRequiresAnAuthenticatedUploader(t *testing.T) {
	service := &paymentSlipServiceStub{}
	router := chi.NewRouter()
	NewPaymentSlipHandler(service).Register(router)

	body, contentType := paymentSlipMultipart(t, "file", "slip.png", pngBytes)
	response := servePaymentSlipRequest(router, body, contentType, false)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if apiError := decodePOSAPIError(t, response); apiError.Code != models.ProjectErrorAuthenticationRequired {
		t.Fatalf("error code = %s", apiError.Code)
	}
	if service.calls != 0 {
		t.Fatal("an unauthenticated upload reached the service")
	}
}

func paymentSlipRouter(service PaymentSlipService) http.Handler {
	router := chi.NewRouter()
	router.Use(middlewares.Authenticate(posCheckoutVerifierStub{}))
	NewPaymentSlipHandler(service).Register(router)
	return router
}

func paymentSlipMultipart(t *testing.T, field, filename string, content []byte) ([]byte, string) {
	t.Helper()
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return buffer.Bytes(), writer.FormDataContentType()
}

func servePaymentSlipRequest(router http.Handler, body []byte, contentType string, authenticated bool) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/upload/payment-slips", bytes.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	if authenticated {
		request.Header.Set("Authorization", "Bearer staff-token")
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

type paymentSlipServiceStub struct {
	slip    models.PaymentSlip
	err     error
	calls   int
	upload  usecases.PaymentSlipUpload
	content string
}

func (stub *paymentSlipServiceStub) Upload(_ context.Context, upload usecases.PaymentSlipUpload) (models.PaymentSlip, error) {
	stub.calls++
	stub.upload = upload
	if upload.Content != nil {
		content, err := io.ReadAll(upload.Content)
		if err != nil {
			return models.PaymentSlip{}, err
		}
		stub.content = string(content)
	}
	if stub.err != nil {
		return models.PaymentSlip{}, stub.err
	}

	return stub.slip, nil
}

var _ PaymentSlipService = (*paymentSlipServiceStub)(nil)
