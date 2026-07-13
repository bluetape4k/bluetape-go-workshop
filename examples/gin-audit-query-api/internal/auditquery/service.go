package auditquery

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/bluetape4k/bluetape-go/audit"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Service struct {
	reader audit.HistoryReader
	config ServiceConfig
}

func NewService(reader audit.HistoryReader, config ServiceConfig) (*Service, error) {
	if reader == nil || isNilInterface(reader) {
		return nil, fmt.Errorf("%w: history reader is required", ErrInvalidConfig)
	}
	if config.DefaultLimit <= 0 || config.MaximumLimit <= 0 || config.DefaultLimit > config.MaximumLimit || config.MaximumLimit > 100 {
		return nil, fmt.Errorf("%w: limits must satisfy 0 < default <= maximum <= 100", ErrInvalidConfig)
	}
	return &Service{reader: reader, config: config}, nil
}

func normalizeAggregate(request AggregateRequest) (audit.AggregateID, error) {
	aggregateType := strings.TrimSpace(request.Type)
	aggregateID := strings.TrimSpace(request.ID)
	if !identifierPattern.MatchString(aggregateType) || !identifierPattern.MatchString(aggregateID) {
		return audit.AggregateID{}, fmt.Errorf("%w: aggregate is invalid", ErrInvalidRequest)
	}
	aggregate, err := audit.NewAggregateID(aggregateType, aggregateID)
	if err != nil {
		return audit.AggregateID{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	return aggregate, nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
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
