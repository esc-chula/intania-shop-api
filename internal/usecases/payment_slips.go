package usecases

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/storage"
)

var (
	// ErrPaymentSlipUnsupportedType reports an upload that is not one of the
	// accepted slip image types.
	ErrPaymentSlipUnsupportedType = errors.New("unsupported payment slip type")
	// ErrPaymentSlipTooLarge reports an upload above the slip size limit.
	ErrPaymentSlipTooLarge = errors.New("payment slip is too large")
	// ErrPaymentSlipEmpty reports an upload with no content.
	ErrPaymentSlipEmpty = errors.New("payment slip is empty")
)

// paymentSlipContentTypes is the accepted set of slip image types.
var paymentSlipContentTypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

// paymentSlipFolder is the object storage prefix for uploaded slips.
const paymentSlipFolder = "payment-slips"

const paymentSlipCleanupTimeout = 5 * time.Second

// PaymentSlipRecorder persists one uploaded slip so that checkout can verify
// the object key it receives was issued by this server.
type PaymentSlipRecorder interface {
	RecordPaymentSlip(context.Context, models.PaymentSlip, int64) error
}

// PaymentSlipUpload is one slip file offered by an administrator. ContentType
// and Size must describe the bytes in Content: the HTTP adapter sniffs the
// type from the file itself and takes the size the multipart reader measured,
// so neither is a value the client declared.
type PaymentSlipUpload struct {
	Filename    string
	ContentType string
	Size        int64
	Content     io.Reader
	UploadedBy  int64
}

// PaymentSlipService stores QR payment slips and records them as trusted
// objects.
type PaymentSlipService struct {
	uploader storage.Uploader
	slips    PaymentSlipRecorder
}

// NewPaymentSlipService constructs a payment slip service.
func NewPaymentSlipService(uploader storage.Uploader, slips PaymentSlipRecorder) *PaymentSlipService {
	return &PaymentSlipService{uploader: uploader, slips: slips}
}

// Upload validates, stores, and records one payment slip. The returned object
// key is the only slip reference a checkout request can use.
func (service *PaymentSlipService) Upload(ctx context.Context, upload PaymentSlipUpload) (models.PaymentSlip, error) {
	if _, supported := paymentSlipContentTypes[upload.ContentType]; !supported {
		return models.PaymentSlip{}, fmt.Errorf("%w: %s", ErrPaymentSlipUnsupportedType, upload.ContentType)
	}
	if upload.Size <= 0 {
		return models.PaymentSlip{}, ErrPaymentSlipEmpty
	}
	if upload.Size > models.PaymentSlipMaxBytes {
		return models.PaymentSlip{}, ErrPaymentSlipTooLarge
	}

	object, err := service.uploader.Upload(ctx, paymentSlipFolder, upload.Filename, upload.ContentType, upload.Content)
	if err != nil {
		return models.PaymentSlip{}, fmt.Errorf("store payment slip: %w", err)
	}

	slip := models.PaymentSlip{
		ObjectKey:   object.ObjectName,
		URL:         object.URL,
		ContentType: upload.ContentType,
		Size:        upload.Size,
	}
	if err := service.slips.RecordPaymentSlip(ctx, slip, upload.UploadedBy); err != nil {
		recordingErr := fmt.Errorf("record payment slip: %w", err)
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), paymentSlipCleanupTimeout)
		defer cancel()
		if cleanupErr := service.uploader.Delete(cleanupCtx, object.ObjectName); cleanupErr != nil {
			return models.PaymentSlip{}, errors.Join(recordingErr, fmt.Errorf("delete unrecorded payment slip: %w", cleanupErr))
		}
		return models.PaymentSlip{}, recordingErr
	}

	return slip, nil
}
