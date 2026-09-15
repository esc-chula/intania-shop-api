package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/esc-chula/intania-shop-api/internal/middlewares"
	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/usecases"
	"github.com/go-chi/chi/v5"
)

// PaymentSlipService is the application surface needed by the payment slip
// HTTP adapter.
type PaymentSlipService interface {
	Upload(context.Context, usecases.PaymentSlipUpload) (models.PaymentSlip, error)
}

// PaymentSlipHandler serves the trusted QR payment slip upload.
type PaymentSlipHandler struct {
	slips PaymentSlipService
}

// NewPaymentSlipHandler constructs a payment slip HTTP adapter.
func NewPaymentSlipHandler(slips PaymentSlipService) *PaymentSlipHandler {
	return &PaymentSlipHandler{slips: slips}
}

// Register mounts the payment slip route on an already-authenticated router.
func (handler *PaymentSlipHandler) Register(router chi.Router) {
	router.Post("/upload/payment-slips", handler.upload)
}

// paymentSlipFormLimit leaves room for the multipart envelope around a slip of
// the maximum accepted size. The exact file size rule is enforced on the part
// itself by the use case.
const paymentSlipFormLimit = models.PaymentSlipMaxBytes + (1 << 20)

func (handler *PaymentSlipHandler) upload(writer http.ResponseWriter, request *http.Request) {
	identity, ok := middlewares.IdentityFromContext(request.Context())
	if !ok {
		writeProjectError(writer, request, http.StatusUnauthorized,
			models.ProjectErrorAuthenticationRequired, "Authentication required")
		return
	}

	request.Body = http.MaxBytesReader(writer, request.Body, paymentSlipFormLimit)
	if err := request.ParseMultipartForm(models.PaymentSlipMaxBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeProjectError(writer, request, http.StatusRequestEntityTooLarge,
				models.ProjectErrorInvalidPaymentSlip, "Payment slip must not be larger than 10 MiB")
			return
		}
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorInvalidPaymentSlip,
			"Request must be a multipart/form-data upload")
		return
	}
	defer func() { _ = request.MultipartForm.RemoveAll() }()

	files := request.MultipartForm.File["file"]
	if len(files) != 1 || len(request.MultipartForm.File) != 1 {
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorInvalidPaymentSlip,
			"The upload must carry exactly one file, in the file field")
		return
	}

	opened, err := files[0].Open()
	if err != nil {
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorInvalidPaymentSlip,
			"Payment slip could not be read")
		return
	}
	defer func() { _ = opened.Close() }()

	// The content type is detected from the bytes rather than trusted from the
	// client-supplied part header.
	head := make([]byte, 512)
	read, err := io.ReadFull(opened, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorInvalidPaymentSlip,
			"Payment slip could not be read")
		return
	}
	head = head[:read]

	slip, err := handler.slips.Upload(request.Context(), usecases.PaymentSlipUpload{
		Filename:    files[0].Filename,
		ContentType: http.DetectContentType(head),
		Size:        files[0].Size,
		Content:     io.MultiReader(bytes.NewReader(head), opened),
		UploadedBy:  identity.UserID,
	})
	if err != nil {
		writePaymentSlipError(writer, request, err)
		return
	}

	writeSuccess(writer, http.StatusCreated, slip)
}

func writePaymentSlipError(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, usecases.ErrPaymentSlipUnsupportedType):
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorInvalidPaymentSlip,
			"Payment slip must be a JPEG, PNG, or WebP image")
	case errors.Is(err, usecases.ErrPaymentSlipEmpty):
		writeProjectError(writer, request, http.StatusBadRequest, models.ProjectErrorInvalidPaymentSlip,
			"Payment slip is empty")
	case errors.Is(err, usecases.ErrPaymentSlipTooLarge):
		writeProjectError(writer, request, http.StatusRequestEntityTooLarge,
			models.ProjectErrorInvalidPaymentSlip, "Payment slip must not be larger than 10 MiB")
	default:
		writeProjectError(writer, request, http.StatusInternalServerError, models.ProjectErrorInternal,
			"Unable to upload payment slip")
	}
}
