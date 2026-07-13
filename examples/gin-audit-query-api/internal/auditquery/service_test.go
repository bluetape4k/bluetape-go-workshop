package auditquery

import (
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/audit"
)

func TestDefaultServiceConfig(t *testing.T) {
	config := DefaultServiceConfig()
	if config.DefaultLimit != 20 || config.MaximumLimit != 100 {
		t.Fatalf("DefaultServiceConfig() = %+v", config)
	}
}

func TestNewServiceRejectsInvalidConfig(t *testing.T) {
	repository := audit.NewMemoryRepository()
	tests := []struct {
		name   string
		reader audit.HistoryReader
		config ServiceConfig
	}{
		{name: "nil reader", reader: nil, config: DefaultServiceConfig()},
		{name: "zero default", reader: repository, config: ServiceConfig{MaximumLimit: 100}},
		{name: "negative default", reader: repository, config: ServiceConfig{DefaultLimit: -1, MaximumLimit: 100}},
		{name: "zero maximum", reader: repository, config: ServiceConfig{DefaultLimit: 20}},
		{name: "default above maximum", reader: repository, config: ServiceConfig{DefaultLimit: 21, MaximumLimit: 20}},
		{name: "maximum above hard bound", reader: repository, config: ServiceConfig{DefaultLimit: 20, MaximumLimit: 101}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := NewService(test.reader, test.config)
			if service != nil || !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("NewService() service = %v, err = %v", service, err)
			}
		})
	}
}

func TestNewServiceRejectsTypedNilReader(t *testing.T) {
	var repository *audit.MemoryRepository
	service, err := NewService(repository, DefaultServiceConfig())
	if service != nil || !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewService() service = %v, err = %v", service, err)
	}
}

func TestNormalizeAggregate(t *testing.T) {
	tests := []struct {
		name    string
		request AggregateRequest
		want    audit.AggregateID
		wantErr error
	}{
		{name: "valid", request: AggregateRequest{Type: " order ", ID: " order-1 "}, want: audit.AggregateID{Type: "order", ID: "order-1"}},
		{name: "blank type", request: AggregateRequest{ID: "order-1"}, wantErr: ErrInvalidRequest},
		{name: "blank id", request: AggregateRequest{Type: "order"}, wantErr: ErrInvalidRequest},
		{name: "path separator", request: AggregateRequest{Type: "order", ID: "tenant/order-1"}, wantErr: ErrInvalidRequest},
		{name: "percent escape", request: AggregateRequest{Type: "order", ID: "order%2F1"}, wantErr: ErrInvalidRequest},
		{name: "too long", request: AggregateRequest{Type: "order", ID: "a" + string(make([]byte, 128))}, wantErr: ErrInvalidRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeAggregate(test.request)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("normalizeAggregate() err = %v, want %v", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("normalizeAggregate() = %+v, want %+v", got, test.want)
			}
		})
	}
}
