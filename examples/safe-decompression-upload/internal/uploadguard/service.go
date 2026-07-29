// Package uploadguard 는 안전한 압축 해제 업로드 예제를 구현한다.
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
	// ErrInvalidRequest 는 HTTP 헤더 또는 업로드 필드가 유효하지 않음을 나타낸다.
	ErrInvalidRequest = errors.New("uploadguard: invalid request")
	// ErrUnsupportedCompression 은 알 수 없는 압축 알고리즘을 나타낸다.
	ErrUnsupportedCompression = errors.New("uploadguard: unsupported compression")
	// ErrMalformedCompressedPayload 는 선택된 압축기가 디코딩할 수 없는 바이트를 나타낸다.
	ErrMalformedCompressedPayload = errors.New("uploadguard: malformed compressed payload")
	// ErrDecompressedPayloadTooLarge 는 압축 해제 후 payload 가 설정 한도를 넘었음을 나타낸다.
	ErrDecompressedPayloadTooLarge = errors.New("uploadguard: decompressed payload too large")
	// ErrCompressedRequestTooLarge 는 압축된 HTTP 요청 본문이 전송 한도를 넘었음을 나타낸다.
	ErrCompressedRequestTooLarge = errors.New("uploadguard: compressed request too large")
)

// Config 는 업로드 guard 의 본문 한도를 제어한다.
type Config struct {
	CompressedBodyLimitBytes   int64
	DecompressedBodyLimitBytes int64
}

// Service 는 업로드 압축 해제 정책을 소유한다.
type Service struct {
	compressedBodyLimit   int64
	decompressedBodyLimit int64
	compressors           map[string]compression.Compressor
}

// UploadDocument 는 압축 해제 후 신뢰되는 JSON 형태다.
type UploadDocument struct {
	DocumentID string `json:"document_id"`
	Tenant     string `json:"tenant"`
	Body       string `json:"body"`
}

// IngestRequest 는 압축 업로드 하나를 서비스로 전달한다.
type IngestRequest struct {
	Algorithm string
	Data      []byte
}

// IngestResponse 는 안정적으로 공개되는 업로드 결과다.
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

// ErrorResponse 는 안정적으로 공개되는 HTTP 오류 형태다.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// NewService 는 업로드 압축 해제 guard 를 생성한다.
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

// CompressedBodyLimitBytes 는 HTTP 압축 요청 본문 한도를 반환한다.
func (s *Service) CompressedBodyLimitBytes() int64 {
	if s == nil || s.compressedBodyLimit == 0 {
		return defaultCompressedBodyLimit
	}
	return s.compressedBodyLimit
}

// DecompressedBodyLimitBytes 는 압축 해제 후 payload 한도를 반환한다.
func (s *Service) DecompressedBodyLimitBytes() int64 {
	if s == nil || s.decompressedBodyLimit == 0 {
		return defaultDecompressedBodyLimit
	}
	return s.decompressedBodyLimit
}

// Ingest 는 업로드 하나를 압축 해제하고 한도 검사, 검증, 응답 변환을 수행한다.
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

// NewRouter 는 예제용 HTTP 라우터를 생성한다.
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
