package usecases

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/esc-chula/intania-shop-api/internal/models"
	"github.com/esc-chula/intania-shop-api/internal/storage"
)

func TestPaymentSlipServiceStoresAndRecordsATrustedSlip(t *testing.T) {
	uploader := &paymentSlipUploaderStub{
		object: storage.Object{
			ObjectName: "payment-slips/1757000000-slip.png",
			URL:        "https://storage.googleapis.com/bucket/payment-slips/1757000000-slip.png",
		},
	}
	recorder := &paymentSlipRecorderStub{}
	service := NewPaymentSlipService(uploader, recorder)

	slip, err := service.Upload(context.Background(), PaymentSlipUpload{
		Filename:    "slip.png",
		ContentType: "image/png",
		Size:        2048,
		Content:     strings.NewReader("image bytes"),
		UploadedBy:  42,
	})
	if err != nil {
		t.Fatal(err)
	}

	if slip.ObjectKey != uploader.object.ObjectName || slip.URL != uploader.object.URL {
		t.Fatalf("slip object = %+v", slip)
	}
	if slip.ContentType != "image/png" || slip.Size != 2048 {
		t.Fatalf("slip metadata = %+v", slip)
	}
	if uploader.folder != "payment-slips" || uploader.filename != "slip.png" || uploader.body != "image bytes" {
		t.Fatalf("upload = %+v", uploader)
	}
	if recorder.calls != 1 || recorder.uploadedBy != 42 || recorder.slip != slip {
		t.Fatalf("recorded slip = %+v by %d over %d calls", recorder.slip, recorder.uploadedBy, recorder.calls)
	}
}

func TestPaymentSlipServiceRejectsUnacceptableUploadsBeforeStoringThem(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		size        int64
		want        error
	}{
		{name: "PDF", contentType: "application/pdf", size: 1024, want: ErrPaymentSlipUnsupportedType},
		{name: "GIF", contentType: "image/gif", size: 1024, want: ErrPaymentSlipUnsupportedType},
		{name: "empty", contentType: "image/png", size: 0, want: ErrPaymentSlipEmpty},
		{name: "above 10 MiB", contentType: "image/jpeg", size: models.PaymentSlipMaxBytes + 1, want: ErrPaymentSlipTooLarge},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uploader := &paymentSlipUploaderStub{}
			recorder := &paymentSlipRecorderStub{}
			service := NewPaymentSlipService(uploader, recorder)

			_, err := service.Upload(context.Background(), PaymentSlipUpload{
				Filename:    "slip",
				ContentType: test.contentType,
				Size:        test.size,
				Content:     strings.NewReader("image bytes"),
				UploadedBy:  42,
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if uploader.calls != 0 || recorder.calls != 0 {
				t.Fatal("a rejected slip was stored or recorded")
			}
		})
	}
}

func TestPaymentSlipServiceAcceptsEveryDocumentedImageType(t *testing.T) {
	for _, contentType := range []string{"image/jpeg", "image/png", "image/webp"} {
		t.Run(contentType, func(t *testing.T) {
			service := NewPaymentSlipService(&paymentSlipUploaderStub{}, &paymentSlipRecorderStub{})
			if _, err := service.Upload(context.Background(), PaymentSlipUpload{
				Filename:    "slip",
				ContentType: contentType,
				Size:        models.PaymentSlipMaxBytes,
				Content:     strings.NewReader("image bytes"),
				UploadedBy:  42,
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPaymentSlipServiceReportsStorageAndRecordingFailures(t *testing.T) {
	storageFailure := errors.New("bucket unavailable")
	service := NewPaymentSlipService(&paymentSlipUploaderStub{err: storageFailure}, &paymentSlipRecorderStub{})
	if _, err := service.Upload(context.Background(), paymentSlipTestUpload()); !errors.Is(err, storageFailure) {
		t.Fatalf("storage error = %v", err)
	}

	recordFailure := errors.New("database unavailable")
	service = NewPaymentSlipService(&paymentSlipUploaderStub{}, &paymentSlipRecorderStub{err: recordFailure})
	if _, err := service.Upload(context.Background(), paymentSlipTestUpload()); !errors.Is(err, recordFailure) {
		t.Fatalf("recording error = %v", err)
	}
}

func paymentSlipTestUpload() PaymentSlipUpload {
	return PaymentSlipUpload{
		Filename:    "slip.png",
		ContentType: "image/png",
		Size:        1024,
		Content:     strings.NewReader("image bytes"),
		UploadedBy:  42,
	}
}

type paymentSlipUploaderStub struct {
	object   storage.Object
	err      error
	calls    int
	folder   string
	filename string
	body     string
}

func (stub *paymentSlipUploaderStub) Upload(_ context.Context, folder, filename, _ string, source io.Reader) (storage.Object, error) {
	stub.calls++
	stub.folder = folder
	stub.filename = filename
	if stub.err != nil {
		return storage.Object{}, stub.err
	}

	body, err := io.ReadAll(source)
	if err != nil {
		return storage.Object{}, err
	}
	stub.body = string(body)

	return stub.object, nil
}

type paymentSlipRecorderStub struct {
	err        error
	calls      int
	slip       models.PaymentSlip
	uploadedBy int64
}

func (stub *paymentSlipRecorderStub) RecordPaymentSlip(_ context.Context, slip models.PaymentSlip, uploadedBy int64) error {
	stub.calls++
	stub.slip = slip
	stub.uploadedBy = uploadedBy
	return stub.err
}
