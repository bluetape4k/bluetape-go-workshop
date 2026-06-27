package uploadguard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/compression"
	"github.com/gin-gonic/gin"
)

func TestServiceIngestsValidCompressedPayload(t *testing.T) {
	service := newTestService(t, 512, 1024)
	payload := uploadJSON(t, UploadDocument{
		DocumentID: "doc-1001",
		Tenant:     "checkout",
		Body:       "invoice upload",
	})
	compressed := compress(t, compression.Gzip(), payload)

	response, err := service.Ingest(context.Background(), IngestRequest{Algorithm: "gzip", Data: compressed})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if response.Decision != "accepted" || response.DocumentID != "doc-1001" || response.Tenant != "checkout" {
		t.Fatalf("response = %+v", response)
	}
	if response.CompressedBytes != len(compressed) || response.DecompressedBytes != len(payload) {
		t.Fatalf("sizes = compressed %d decompressed %d", response.CompressedBytes, response.DecompressedBytes)
	}
	if !response.AuthoritativeScanStillNeeded {
		t.Fatal("AuthoritativeScanStillNeeded = false")
	}
}

func TestServiceSupportsZstdPayloads(t *testing.T) {
	service := newTestService(t, 512, 1024)
	payload := uploadJSON(t, UploadDocument{DocumentID: "doc-zstd", Tenant: "checkout", Body: "zstd upload"})
	compressed := compress(t, compression.Zstd(), payload)

	response, err := service.Ingest(context.Background(), IngestRequest{Algorithm: "zstd", Data: compressed})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if response.Algorithm != "zstd" {
		t.Fatalf("algorithm = %q", response.Algorithm)
	}
}

func TestServiceRejectsMalformedCompressedPayload(t *testing.T) {
	service := newTestService(t, 512, 1024)
	_, err := service.Ingest(context.Background(), IngestRequest{Algorithm: "gzip", Data: []byte("not gzip")})
	if !errors.Is(err, ErrMalformedCompressedPayload) {
		t.Fatalf("error = %v, want ErrMalformedCompressedPayload", err)
	}
}

func TestServicePreservesDecompressLimitTypedError(t *testing.T) {
	service := newTestService(t, 512, 128)
	payload := uploadJSON(t, UploadDocument{
		DocumentID: "doc-large",
		Tenant:     "checkout",
		Body:       strings.Repeat("x", 512),
	})
	compressed := compress(t, compression.Gzip(), payload)

	_, err := service.Ingest(context.Background(), IngestRequest{Algorithm: "gzip", Data: compressed})
	if !errors.Is(err, ErrDecompressedPayloadTooLarge) {
		t.Fatalf("error = %v, want ErrDecompressedPayloadTooLarge", err)
	}
	if !errors.Is(err, compression.ErrDecompressedSizeExceeded) {
		t.Fatalf("error = %v, want compression.ErrDecompressedSizeExceeded", err)
	}
}

func TestServiceHonorsCancelledContext(t *testing.T) {
	service := newTestService(t, 512, 1024)
	payload := uploadJSON(t, UploadDocument{DocumentID: "doc-cancel", Tenant: "checkout", Body: "cancel"})
	compressed := compress(t, compression.Gzip(), payload)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.Ingest(ctx, IngestRequest{Algorithm: "gzip", Data: compressed})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestServiceRejectsInvalidPayloadAndUnsupportedAlgorithm(t *testing.T) {
	service := newTestService(t, 512, 1024)
	invalidJSON := compress(t, compression.Gzip(), []byte(`{"document_id":"doc-1","tenant":"checkout","body":""}`))
	if _, err := service.Ingest(context.Background(), IngestRequest{Algorithm: "gzip", Data: invalidJSON}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid JSON shape error = %v, want ErrInvalidRequest", err)
	}
	if _, err := service.Ingest(context.Background(), IngestRequest{Algorithm: "br", Data: invalidJSON}); !errors.Is(err, ErrUnsupportedCompression) {
		t.Fatalf("unsupported error = %v, want ErrUnsupportedCompression", err)
	}
}

func TestRouterMapsResponsesAndErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newTestService(t, 120, 160)
	handler := NewRouter(service)

	validPayload := uploadJSON(t, UploadDocument{DocumentID: "doc-http", Tenant: "checkout", Body: "ok"})
	validCompressed := compress(t, compression.Gzip(), validPayload)
	okResponse := postCompressed(t, handler, "gzip", validCompressed)
	if okResponse.Code != http.StatusAccepted {
		t.Fatalf("valid status = %d, want %d: %s", okResponse.Code, http.StatusAccepted, okResponse.Body.String())
	}

	malformedResponse := postCompressed(t, handler, "gzip", []byte("not gzip"))
	assertHTTPError(t, malformedResponse, http.StatusBadRequest, "malformed_compressed_payload")

	largePayload := uploadJSON(t, UploadDocument{DocumentID: "doc-large", Tenant: "checkout", Body: strings.Repeat("x", 256)})
	largeCompressed := compress(t, compression.Gzip(), largePayload)
	largeResponse := postCompressed(t, handler, "gzip", largeCompressed)
	assertHTTPError(t, largeResponse, http.StatusRequestEntityTooLarge, "decompressed_payload_too_large")

	unsupportedResponse := postCompressed(t, handler, "br", validCompressed)
	assertHTTPError(t, unsupportedResponse, http.StatusBadRequest, "unsupported_compression")

	oversizedCompressedResponse := postCompressed(t, handler, "gzip", bytes.Repeat([]byte("x"), 256))
	assertHTTPError(t, oversizedCompressedResponse, http.StatusRequestEntityTooLarge, "compressed_request_too_large")
}

func TestRouterMapsCancelledRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newTestService(t, 512, 1024)
	handler := NewRouter(service)
	payload := uploadJSON(t, UploadDocument{DocumentID: "doc-cancel", Tenant: "checkout", Body: "cancel"})
	compressed := compress(t, compression.Gzip(), payload)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/uploads/compressed", bytes.NewReader(compressed))
	req.Header.Set("X-Compression-Algorithm", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertHTTPError(t, rec, http.StatusRequestTimeout, "request_cancelled")
}

func TestRouterReportsLimitsAndHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newTestService(t, 512, 1024)
	handler := NewRouter(service)

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d", health.Code)
	}

	limits := httptest.NewRecorder()
	handler.ServeHTTP(limits, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/uploads/limits", nil))
	if limits.Code != http.StatusOK {
		t.Fatalf("limits status = %d", limits.Code)
	}
	if !strings.Contains(limits.Body.String(), `"gzip"`) || !strings.Contains(limits.Body.String(), `"zstd"`) {
		t.Fatalf("limits body = %s", limits.Body.String())
	}
}

func newTestService(t *testing.T, compressedLimit, decompressedLimit int64) *Service {
	t.Helper()
	service, err := NewService(Config{
		CompressedBodyLimitBytes:   compressedLimit,
		DecompressedBodyLimitBytes: decompressedLimit,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func uploadJSON(t *testing.T, document UploadDocument) []byte {
	t.Helper()
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return data
}

func compress(t *testing.T, compressor compression.Compressor, payload []byte) []byte {
	t.Helper()
	data, err := compressor.Compress(payload)
	if err != nil {
		t.Fatalf("%s Compress() error = %v", compressor.Name(), err)
	}
	return data
}

func postCompressed(t *testing.T, handler http.Handler, algorithm string, payload []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/uploads/compressed", bytes.NewReader(payload))
	req.Header.Set("X-Compression-Algorithm", algorithm)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func assertHTTPError(t *testing.T, response *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d: %s", response.Code, wantStatus, response.Body.String())
	}
	var body ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v: %s", err, response.Body.String())
	}
	if body.ErrorCode != wantCode {
		t.Fatalf("error code = %q, want %q: %s", body.ErrorCode, wantCode, response.Body.String())
	}
}
