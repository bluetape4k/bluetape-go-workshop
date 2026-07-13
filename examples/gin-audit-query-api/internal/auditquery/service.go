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

func (s *Service) Search(ctx context.Context, request SearchRequest) (SearchResponse, error) {
	if !s.ready() {
		return SearchResponse{}, ErrInvalidConfig
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}
	aggregate, err := normalizeAggregate(request.Aggregate)
	if err != nil {
		return SearchResponse{}, err
	}
	limit := request.Limit
	if limit == 0 {
		limit = s.config.DefaultLimit
	}
	if limit < 0 || limit > s.config.MaximumLimit {
		cause := audit.ValidationError{Kind: audit.ErrInvalidQuery, Field: "limit", Value: request.Limit}
		return SearchResponse{}, fmt.Errorf("%w: %w", ErrInvalidRequest, cause)
	}
	query, err := (audit.Query{
		Aggregate:      &aggregate,
		FromRevision:   request.FromRevision,
		ToRevision:     request.ToRevision,
		FromRecordedAt: request.FromRecordedAt,
		ToRecordedAt:   request.ToRecordedAt,
		NewestFirst:    request.NewestFirst,
		Limit:          limit + 1,
	}).Validate()
	if err != nil {
		return SearchResponse{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	entries, err := s.reader.Find(ctx, query)
	if err != nil {
		return SearchResponse{}, fmt.Errorf("find audit history: %w", err)
	}
	response := SearchResponse{
		Entries: entries,
		Page:    Page{Limit: limit},
	}
	if response.Entries == nil {
		response.Entries = []audit.Entry{}
	}
	if len(response.Entries) <= limit {
		return response, nil
	}
	nextRevision := response.Entries[limit].Revision
	response.Entries = response.Entries[:limit]
	response.Page.HasMore = true
	response.Page.Next = &NextPage{}
	if request.NewestFirst {
		response.Page.Next.ToRevision = nextRevision
	} else {
		response.Page.Next.FromRevision = nextRevision
	}
	return response, nil
}

func (s *Service) Get(ctx context.Context, request AggregateRequest, revision audit.Revision) (audit.Entry, error) {
	if !s.ready() {
		return audit.Entry{}, ErrInvalidConfig
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return audit.Entry{}, err
	}
	aggregate, err := normalizeAggregate(request)
	if err != nil {
		return audit.Entry{}, err
	}
	if err := revision.Validate(); err != nil {
		return audit.Entry{}, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	entries, err := s.reader.Find(ctx, audit.Query{
		Aggregate:    &aggregate,
		FromRevision: revision,
		ToRevision:   revision,
		Limit:        1,
	})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("find audit entry: %w", err)
	}
	if len(entries) == 0 {
		return audit.Entry{}, ErrEntryNotFound
	}
	return entries[0], nil
}

func (s *Service) ready() bool {
	return s != nil && s.reader != nil && !isNilInterface(s.reader) && s.config.DefaultLimit > 0 && s.config.MaximumLimit >= s.config.DefaultLimit
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
