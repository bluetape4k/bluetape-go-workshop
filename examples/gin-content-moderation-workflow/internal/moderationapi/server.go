package moderationapi

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
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// Workflow is the narrow service contract used by the Gin boundary.
type Workflow interface {
	Create(context.Context, CreateRequest) (Record, error)
	Get(context.Context, string) (Record, error)
	Search(context.Context, SearchRequest) (SearchResponse, error)
}

type publicError struct {
	Status  int
	Code    string
	Message string
}

var (
	errRequestTooLarge            = errors.New("request body is too large")
	errUnsupportedMediaType       = errors.New("unsupported media type")
	errUnsupportedContentEncoding = errors.New("unsupported content encoding")
)

// NewEngine builds the strict, bounded Gin HTTP adapter.
func NewEngine(workflow Workflow, config HTTPConfig, logger *log.Logger) (*gin.Engine, error) {
	if workflow == nil || isNilInterface(workflow) || logger == nil || config.MaximumBodyBytes <= 0 || config.RequestTimeout <= 0 {
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

	adapter := httpAdapter{workflow: workflow, config: config, logger: logger}
	engine.GET("/healthz", adapter.health)
	engine.POST("/moderation/records", adapter.create)
	engine.GET("/moderation/records/:content_id", adapter.get)
	engine.POST("/moderation/records/search", adapter.search)
	return engine, nil
}

type httpAdapter struct {
	workflow Workflow
	config   HTTPConfig
	logger   *log.Logger
}

func (a httpAdapter) health(c *gin.Context) {
	defer closeRequestBody(c.Request.Body)
	a.respond(c, http.StatusOK, "ok", gin.H{"status": "ok"}, time.Now())
}

func (a httpAdapter) create(c *gin.Context) {
	started := time.Now()
	defer closeRequestBody(c.Request.Body)
	var request CreateRequest
	if err := a.decodeJSON(c, &request); err != nil {
		a.respondDecodeError(c, err, started)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), a.config.RequestTimeout)
	defer cancel()
	record, err := a.workflow.Create(ctx, request)
	if err != nil {
		a.respondWorkflowError(c, err, started)
		return
	}
	if a.stopAfterWorkflow(ctx, c, started) {
		return
	}
	a.respond(c, http.StatusCreated, "ok", record, started)
}

func (a httpAdapter) get(c *gin.Context) {
	started := time.Now()
	defer closeRequestBody(c.Request.Body)
	contentID := strings.TrimSpace(c.Param("content_id"))
	if !contentIDPattern.MatchString(contentID) {
		a.respondError(c, mapWorkflowError(ErrInvalidRequest), started)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), a.config.RequestTimeout)
	defer cancel()
	record, err := a.workflow.Get(ctx, contentID)
	if err != nil {
		a.respondWorkflowError(c, err, started)
		return
	}
	if a.stopAfterWorkflow(ctx, c, started) {
		return
	}
	a.respond(c, http.StatusOK, "ok", record, started)
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
	response, err := a.workflow.Search(ctx, request)
	if err != nil {
		a.respondWorkflowError(c, err, started)
		return
	}
	if a.stopAfterWorkflow(ctx, c, started) {
		return
	}
	if response.Hits == nil {
		response.Hits = []SearchHit{}
	}
	a.respond(c, http.StatusOK, "ok", response, started)
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
		a.respondError(c, publicError{Status: http.StatusRequestEntityTooLarge, Code: "request_too_large", Message: "request body is too large"}, started)
	case errors.Is(err, errUnsupportedMediaType):
		a.respondError(c, publicError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "application/json is required"}, started)
	case errors.Is(err, errUnsupportedContentEncoding):
		a.respondError(c, publicError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_content_encoding", Message: "content encoding is unsupported"}, started)
	case errors.Is(err, ErrInvalidRequest):
		a.respondError(c, mapWorkflowError(ErrInvalidRequest), started)
	default:
		a.respondError(c, mapWorkflowError(ErrWorkflow), started)
	}
}

func (a httpAdapter) respondWorkflowError(c *gin.Context, err error, started time.Time) {
	if c.Request.Context().Err() != nil {
		a.log(c, 0, "request_canceled", started)
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		a.respondError(c, publicError{Status: http.StatusRequestTimeout, Code: "request_timeout", Message: "request timed out"}, started)
		return
	}
	a.respondError(c, mapWorkflowError(err), started)
}

func (a httpAdapter) stopAfterWorkflow(workflowContext context.Context, c *gin.Context, started time.Time) bool {
	if c.Request.Context().Err() != nil {
		a.log(c, 0, "request_canceled", started)
		return true
	}
	if errors.Is(workflowContext.Err(), context.DeadlineExceeded) {
		a.respondError(c, publicError{Status: http.StatusRequestTimeout, Code: "request_timeout", Message: "request timed out"}, started)
		return true
	}
	return false
}

func mapWorkflowError(err error) publicError {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return publicError{Status: http.StatusBadRequest, Code: "invalid_request", Message: "request is invalid"}
	case errors.Is(err, ErrDuplicateContentID):
		return publicError{Status: http.StatusConflict, Code: "duplicate_content_id", Message: "content_id already exists"}
	case errors.Is(err, ErrRecordNotFound):
		return publicError{Status: http.StatusNotFound, Code: "record_not_found", Message: "record was not found"}
	case errors.Is(err, ErrStoreCapacity):
		return publicError{Status: http.StatusServiceUnavailable, Code: "store_capacity_reached", Message: "record capacity is reached"}
	default:
		return publicError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "internal error"}
	}
}

func (a httpAdapter) respondError(c *gin.Context, public publicError, started time.Time) {
	a.respond(c, public.Status, public.Code, gin.H{"error": gin.H{"code": public.Code, "message": public.Message}}, started)
}

func (a httpAdapter) respond(c *gin.Context, status int, code string, body any, started time.Time) {
	c.JSON(status, body)
	a.log(c, status, code, started)
}

func (a httpAdapter) log(c *gin.Context, status int, code string, started time.Time) {
	a.logger.Printf("route=%q method=%q status=%d code=%q elapsed=%s", c.FullPath(), c.Request.Method, status, code, time.Since(started))
}

func isNilInterface(value any) bool {
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func closeRequestBody(body io.Closer) {
	_ = body.Close()
}
