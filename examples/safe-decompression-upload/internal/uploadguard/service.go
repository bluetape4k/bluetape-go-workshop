// Package uploadguard implements the safe decompression upload example.
package uploadguard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bluetape4k/bluetape-go/compression"
	"github.com/gin-gonic/gin"
)

const (
	defaultCompressedBodyLimit   int64 = 32 << 10
	defaultDecompressedBodyLimit int64 = 256 << 10
)

var (
	// ErrInvalidRequest reports invalid HTTP headers or upload fields.
	ErrInvalidRequest = errors.New("uploadguard: invalid request")
	// ErrUnsupportedCompression reports an unknown compression algorithm.
	ErrUnsupportedCompression = errors.New("uploadguard: unsupported compression")
	// ErrMalformedCompressedPayload reports bytes that the selected compressor cannot decode.
	ErrMalformedCompressedPayload = errors.New("uploadguard: malformed compressed payload")
	// ErrDecompressedPayloadTooLarge reports an expanded payload beyond the configured limit.
	ErrDecompressedPayloadTooLarge = errors.New("uploadguard: decompressed payload too large")
	// ErrCompressedRequestTooLarge reports a compressed HTTP request body beyond the transport limit.
	ErrCompressedRequestTooLarge = errors.New("uploadguard: compressed request too large")
)

// Config controls body limits for the upload guard.
type Config struct {
	CompressedBodyLimitBytes   int64
	DecompressedBodyLimitBytes int64
}

// Service owns upload decompression policy.
type Service struct {
	compressedBodyLimit   int64
	decompressedBodyLimit int64
	compressors           map[string]compression.Compressor
}

// UploadDocument is the trusted JSON shape after decompression.
type UploadDocument struct {
	DocumentID string `json:"document_id"`
	Tenant     string `json:"tenant"`
	Body       string `json:"body"`
}

// IngestRequest carries one compressed upload into the service.
type IngestRequest struct {
	Algorithm string
	Data      []byte
}

// IngestResponse is the stable public upload result.
type IngestResponse struct {
	DocumentID                   string `json:"document_id"`
	Tenant                       string `json:"tenant"`
	Algorithm                    string `json:"algorithm"`
	Decision                     string `json:"decision"`
	CompressedBytes              int    `json:"compressed_bytes"`
	DecompressedBytes            int    `json:"decompressed_bytes"`
	CompressedBodyLimitBytes     int64  `json:"compressed_body_limit_bytes"`
	DecompressedBodyLimitBytes   int64  `json:"decompressed_body_limit_bytes"`
	PayloadSHA256                string `json:"payload_sha256"`
	AuthoritativeScanStillNeeded bool   `json:"authoritative_scan_still_needed"`
}

// ErrorResponse is the stable public HTTP error shape.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// NewService creates an upload decompression guard.
func NewService(config Config) (*Service, error) {
	compressedLimit := config.CompressedBodyLimitBytes
	if compressedLimit == 0 {
		compressedLimit = defaultCompressedBodyLimit
	}
	decompressedLimit := config.DecompressedBodyLimitBytes
	if decompressedLimit == 0 {
		decompressedLimit = defaultDecompressedBodyLimit
	}
	if compressedLimit < 0 {
		return nil, fmt.Errorf("%w: compressed limit must be non-negative", ErrInvalidRequest)
	}
	if decompressedLimit < 0 {
		return nil, fmt.Errorf("%w: decompressed limit must be non-negative", ErrInvalidRequest)
	}
	return &Service{
		compressedBodyLimit:   compressedLimit,
		decompressedBodyLimit: decompressedLimit,
		compressors: map[string]compression.Compressor{
			"gzip": compression.Gzip(),
			"zstd": compression.Zstd(),
		},
	}, nil
}

// CompressedBodyLimitBytes returns the HTTP compressed request body limit.
func (s *Service) CompressedBodyLimitBytes() int64 {
	if s == nil || s.compressedBodyLimit == 0 {
		return defaultCompressedBodyLimit
	}
	return s.compressedBodyLimit
}

// DecompressedBodyLimitBytes returns the post-decompression payload limit.
func (s *Service) DecompressedBodyLimitBytes() int64 {
	if s == nil || s.decompressedBodyLimit == 0 {
		return defaultDecompressedBodyLimit
	}
	return s.decompressedBodyLimit
}

// Ingest decompresses, bounds, validates, and projects one upload.
func (s *Service) Ingest(ctx context.Context, request IngestRequest) (IngestResponse, error) {
	if err := ctx.Err(); err != nil {
		return IngestResponse{}, err
	}
	if s == nil {
		return IngestResponse{}, fmt.Errorf("%w: service is nil", ErrInvalidRequest)
	}
	algorithm := normalizeAlgorithm(request.Algorithm)
	compressor, ok := s.compressors[algorithm]
	if !ok {
		return IngestResponse{}, fmt.Errorf("%w: %s", ErrUnsupportedCompression, algorithm)
	}
	if len(request.Data) == 0 {
		return IngestResponse{}, fmt.Errorf("%w: compressed body is required", ErrInvalidRequest)
	}

	payload, err := compression.DecompressLimit(compressor, request.Data, s.DecompressedBodyLimitBytes())
	if err != nil {
		return IngestResponse{}, mapDecompressionError(err)
	}
	if err := ctx.Err(); err != nil {
		return IngestResponse{}, err
	}

	var document UploadDocument
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return IngestResponse{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return IngestResponse{}, fmt.Errorf("%w: trailing JSON content", ErrInvalidRequest)
	}
	document.DocumentID = strings.TrimSpace(document.DocumentID)
	document.Tenant = strings.TrimSpace(document.Tenant)
	if document.DocumentID == "" || document.Tenant == "" || document.Body == "" {
		return IngestResponse{}, fmt.Errorf("%w: document_id, tenant, and body are required", ErrInvalidRequest)
	}

	sum := sha256.Sum256(payload)
	return IngestResponse{
		DocumentID:                   document.DocumentID,
		Tenant:                       document.Tenant,
		Algorithm:                    algorithm,
		Decision:                     "accepted",
		CompressedBytes:              len(request.Data),
		DecompressedBytes:            len(payload),
		CompressedBodyLimitBytes:     s.CompressedBodyLimitBytes(),
		DecompressedBodyLimitBytes:   s.DecompressedBodyLimitBytes(),
		PayloadSHA256:                hex.EncodeToString(sum[:]),
		AuthoritativeScanStillNeeded: true,
	}, nil
}

// NewRouter creates the HTTP router for the example.
func NewRouter(service *Service) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/uploads/limits", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"compressed_body_limit_bytes":   service.CompressedBodyLimitBytes(),
			"decompressed_body_limit_bytes": service.DecompressedBodyLimitBytes(),
			"supported_algorithms":          []string{"gzip", "zstd"},
		})
	})
	router.POST("/uploads/compressed", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.CompressedBodyLimitBytes())
		data, err := io.ReadAll(c.Request.Body)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				writeError(c, http.StatusRequestEntityTooLarge, "compressed_request_too_large", "compressed request body exceeds limit")
				return
			}
			writeServiceError(c, err)
			return
		}
		response, err := service.Ingest(c.Request.Context(), IngestRequest{
			Algorithm: compressionHeader(c.Request),
			Data:      data,
		})
		if err != nil {
			writeServiceError(c, err)
			return
		}
		c.JSON(http.StatusAccepted, response)
	})

	return router
}

func normalizeAlgorithm(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "identity" {
		return ""
	}
	return value
}

func compressionHeader(request *http.Request) string {
	if request == nil {
		return ""
	}
	if value := request.Header.Get("X-Compression-Algorithm"); strings.TrimSpace(value) != "" {
		return value
	}
	return request.Header.Get("Content-Encoding")
}

func mapDecompressionError(err error) error {
	if errors.Is(err, compression.ErrDecompressedSizeExceeded) {
		return fmt.Errorf("%w: %w", ErrDecompressedPayloadTooLarge, err)
	}
	return fmt.Errorf("%w: %w", ErrMalformedCompressedPayload, err)
}

func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUnsupportedCompression):
		writeError(c, http.StatusBadRequest, "unsupported_compression", "unsupported compression algorithm")
	case errors.Is(err, ErrDecompressedPayloadTooLarge), errors.Is(err, compression.ErrDecompressedSizeExceeded):
		writeError(c, http.StatusRequestEntityTooLarge, "decompressed_payload_too_large", "decompressed payload exceeds limit")
	case errors.Is(err, ErrCompressedRequestTooLarge):
		writeError(c, http.StatusRequestEntityTooLarge, "compressed_request_too_large", "compressed request body exceeds limit")
	case errors.Is(err, ErrMalformedCompressedPayload):
		writeError(c, http.StatusBadRequest, "malformed_compressed_payload", "compressed payload could not be decoded")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeError(c, http.StatusRequestTimeout, "request_cancelled", "request was cancelled before upload ingestion completed")
	case errors.Is(err, ErrInvalidRequest):
		writeError(c, http.StatusBadRequest, "invalid_upload_payload", "upload payload is invalid")
	default:
		writeError(c, http.StatusInternalServerError, "upload_ingestion_failed", "upload ingestion failed")
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{ErrorCode: code, Message: message})
}
