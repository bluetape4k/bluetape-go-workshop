package auditquery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/gin-gonic/gin"
)

var (
	errRequestTooLarge            = errors.New("request body is too large")
	errUnsupportedMediaType       = errors.New("unsupported media type")
	errUnsupportedContentEncoding = errors.New("unsupported content encoding")
)

type HTTPConfig struct {
	MaximumBodyBytes int64
	RequestTimeout   time.Duration
}

func DefaultHTTPConfig() HTTPConfig {
	return HTTPConfig{MaximumBodyBytes: 32 << 10, RequestTimeout: 2 * time.Second}
}

type QueryService interface {
	Search(context.Context, SearchRequest) (SearchResponse, error)
	Get(context.Context, AggregateRequest, audit.Revision) (audit.Entry, error)
}

func NewEngine(service QueryService, config HTTPConfig, logger *log.Logger) (*gin.Engine, error) {
	if service == nil || isNilInterface(service) || logger == nil || config.MaximumBodyBytes <= 0 || config.RequestTimeout <= 0 {
		return nil, ErrInvalidConfig
	}
	engine := gin.New()
	engine.HandleMethodNotAllowed = true
	engine.RedirectTrailingSlash = false
	engine.UseRawPath = true
	engine.UnescapePathValues = false
	if err := engine.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("%w: disable trusted proxies: %w", ErrInvalidConfig, err)
	}
	adapter := httpAdapter{service: service, config: config, logger: logger}
	engine.NoRoute(adapter.notFound)
	engine.NoMethod(adapter.methodNotAllowed)
	engine.GET("/healthz", adapter.health)
	engine.POST("/audit/history/search", adapter.search)
	engine.GET("/audit/aggregates/:type/:id/revisions/:revision", adapter.detail)
	return engine, nil
}

type httpAdapter struct {
	service QueryService
	config  HTTPConfig
	logger  *log.Logger
}

type publicError struct {
	status  int
	code    string
	message string
}

func (a httpAdapter) health(c *gin.Context) {
	started := time.Now()
	defer closeRequestBody(c.Request.Body)
	a.respond(c, http.StatusOK, "ok", gin.H{"status": "ok"}, started)
}

func (a httpAdapter) search(c *gin.Context) {
	started := time.Now()
	defer closeRequestBody(c.Request.Body)
	var request SearchRequest
	if err := a.decodeJSON(c, &request); err != nil {
		a.respondDecodeError(c, err, started)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), a.config.RequestTimeout)
	defer cancel()
	response, err := a.service.Search(ctx, request)
	if err != nil {
		a.respondServiceError(c, err, started)
		return
	}
	if a.stopAfterService(ctx, c, started) {
		return
	}
	if response.Entries == nil {
		response.Entries = []audit.Entry{}
	}
	a.respond(c, http.StatusOK, "ok", response, started)
}

func (a httpAdapter) detail(c *gin.Context) {
	started := time.Now()
	defer closeRequestBody(c.Request.Body)
	revisionValue, err := strconv.ParseUint(strings.TrimSpace(c.Param("revision")), 10, 64)
	if err != nil || revisionValue == 0 {
		a.respondError(c, publicError{status: http.StatusBadRequest, code: "invalid_audit_query", message: "audit query is invalid"}, started)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), a.config.RequestTimeout)
	defer cancel()
	entry, err := a.service.Get(ctx, AggregateRequest{Type: c.Param("type"), ID: c.Param("id")}, audit.Revision(revisionValue))
	if err != nil {
		a.respondServiceError(c, err, started)
		return
	}
	if a.stopAfterService(ctx, c, started) {
		return
	}
	a.respond(c, http.StatusOK, "ok", gin.H{"entry": entry}, started)
}

func (a httpAdapter) notFound(c *gin.Context) {
	started := time.Now()
	defer closeRequestBody(c.Request.Body)
	a.respondError(c, publicError{status: http.StatusNotFound, code: "route_not_found", message: "route was not found"}, started)
}

func (a httpAdapter) methodNotAllowed(c *gin.Context) {
	started := time.Now()
	defer closeRequestBody(c.Request.Body)
	a.respondError(c, publicError{status: http.StatusMethodNotAllowed, code: "method_not_allowed", message: "method is not allowed"}, started)
}

func (a httpAdapter) decodeJSON(c *gin.Context, target any) error {
	mediaType, parameters, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") || len(parameters) > 1 {
		return errUnsupportedMediaType
	}
	for key, value := range parameters {
		if !strings.EqualFold(key, "charset") || !strings.EqualFold(value, "utf-8") {
			return errUnsupportedMediaType
		}
	}
	encoding := strings.TrimSpace(c.GetHeader("Content-Encoding"))
	if encoding != "" && !strings.EqualFold(encoding, "identity") {
		return errUnsupportedContentEncoding
	}
	limited := http.MaxBytesReader(c.Writer, c.Request.Body, a.config.MaximumBodyBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		var maximum *http.MaxBytesError
		if errors.As(err, &maximum) {
			return errRequestTooLarge
		}
		return fmt.Errorf("read body: %w", err)
	}
	if !utf8.Valid(body) {
		return ErrInvalidRequest
	}
	if err := rejectDuplicateJSONKeys(body); err != nil {
		return ErrInvalidRequest
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrInvalidRequest
	}
	if err := requireJSONEOF(decoder); err != nil {
		return ErrInvalidRequest
	}
	return nil
}

func rejectDuplicateJSONKeys(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	var parseValue func() error
	parseValue = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := make(map[string]struct{})
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return errors.New("object key is not a string")
				}
				if _, exists := seen[key]; exists {
					return errors.New("duplicate object key")
				}
				seen[key] = struct{}{}
				if err := parseValue(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := parseValue(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return errors.New("unexpected closing delimiter")
		}
	}
	if err := parseValue(); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func requireJSONEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func (a httpAdapter) respondDecodeError(c *gin.Context, err error, started time.Time) {
	switch {
	case errors.Is(err, errRequestTooLarge):
		a.respondError(c, publicError{status: http.StatusRequestEntityTooLarge, code: "request_too_large", message: "request body is too large"}, started)
	case errors.Is(err, errUnsupportedMediaType):
		a.respondError(c, publicError{status: http.StatusUnsupportedMediaType, code: "unsupported_media_type", message: "application/json is required"}, started)
	case errors.Is(err, errUnsupportedContentEncoding):
		a.respondError(c, publicError{status: http.StatusUnsupportedMediaType, code: "unsupported_content_encoding", message: "content encoding is unsupported"}, started)
	case errors.Is(err, ErrInvalidRequest):
		a.respondError(c, publicError{status: http.StatusBadRequest, code: "invalid_request", message: "request is invalid"}, started)
	default:
		a.respondError(c, publicError{status: http.StatusInternalServerError, code: "internal_error", message: "internal error"}, started)
	}
}

func (a httpAdapter) respondServiceError(c *gin.Context, err error, started time.Time) {
	if c.Request.Context().Err() != nil {
		a.log(c, 0, "request_canceled", started)
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		a.respondError(c, publicError{status: http.StatusRequestTimeout, code: "request_timeout", message: "request timed out"}, started)
		return
	}
	a.respondError(c, mapServiceError(err), started)
}

func (a httpAdapter) stopAfterService(serviceContext context.Context, c *gin.Context, started time.Time) bool {
	if c.Request.Context().Err() != nil {
		a.log(c, 0, "request_canceled", started)
		return true
	}
	if errors.Is(serviceContext.Err(), context.DeadlineExceeded) {
		a.respondError(c, publicError{status: http.StatusRequestTimeout, code: "request_timeout", message: "request timed out"}, started)
		return true
	}
	return false
}

func mapServiceError(err error) publicError {
	switch {
	case errors.Is(err, audit.ErrInvalidAggregateID), errors.Is(err, audit.ErrInvalidRevision), errors.Is(err, audit.ErrInvalidQuery):
		return publicError{status: http.StatusBadRequest, code: "invalid_audit_query", message: "audit query is invalid"}
	case errors.Is(err, ErrInvalidRequest):
		return publicError{status: http.StatusBadRequest, code: "invalid_request", message: "request is invalid"}
	case errors.Is(err, ErrEntryNotFound):
		return publicError{status: http.StatusNotFound, code: "audit_entry_not_found", message: "audit entry was not found"}
	default:
		return publicError{status: http.StatusInternalServerError, code: "internal_error", message: "internal error"}
	}
}

func (a httpAdapter) respondError(c *gin.Context, problem publicError, started time.Time) {
	a.respond(c, problem.status, problem.code, gin.H{"error": gin.H{"code": problem.code, "message": problem.message}}, started)
}

func (a httpAdapter) respond(c *gin.Context, status int, code string, body any, started time.Time) {
	c.JSON(status, body)
	a.log(c, status, code, started)
}

func (a httpAdapter) log(c *gin.Context, status int, code string, started time.Time) {
	a.logger.Printf("route=%q method=%q status=%d code=%q elapsed=%s", c.FullPath(), c.Request.Method, status, code, time.Since(started))
}

func closeRequestBody(body io.Closer) {
	_ = body.Close()
}
