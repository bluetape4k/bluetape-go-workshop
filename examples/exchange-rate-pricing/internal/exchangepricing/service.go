// Package exchangepricing 은 provider-backed exchange-rate pricing 예제를 구현한다.
package exchangepricing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/money"
	"github.com/gin-gonic/gin"
)

const (
	maxJSONBodySize         = 8 << 10
	defaultOperationTimeout = 750 * time.Millisecond
)

var (
	// ErrInvalidRequest 는 요청 JSON 또는 필드가 유효하지 않음을 나타낸다.
	ErrInvalidRequest = errors.New("exchangepricing: invalid request")
	// ErrInvalidMoney 는 currency 또는 decimal money 입력이 유효하지 않음을 나타낸다.
	ErrInvalidMoney = errors.New("exchangepricing: invalid money")
	// ErrInvalidLocale 은 명시적으로 지원되는 currency가 없는 locale을 나타낸다.
	ErrInvalidLocale = errors.New("exchangepricing: invalid locale")
	// ErrCurrencyMismatch 는 line currency가 quote base currency와 다름을 나타낸다.
	ErrCurrencyMismatch = errors.New("exchangepricing: currency mismatch")
	// ErrProviderUnavailable 은 원본 provider 세부 정보를 노출하지 않고 exchange-rate provider 실패를 나타낸다.
	ErrProviderUnavailable = errors.New("exchangepricing: exchange-rate provider unavailable")
	// ErrStaleQuote 는 호출자가 허용하지 않은 stale quote metadata를 나타낸다.
	ErrStaleQuote = errors.New("exchangepricing: stale quote")
)

// Service 는 provider-backed rate로 base cart total을 locale display total로 변환한다.
type Service struct {
	provider         money.ExchangeRateProvider
	operationTimeout time.Duration
}

// Option 은 Service를 설정한다.
type Option func(*Service)

// WithOperationTimeout 은 provider I/O 주변에 사용할 deadline을 설정한다.
func WithOperationTimeout(timeout time.Duration) Option {
	return func(service *Service) {
		service.operationTimeout = timeout
	}
}

// QuoteRequest 는 display-pricing quote 하나에 대한 HTTP 및 service 요청이다.
type QuoteRequest struct {
	QuoteID         string            `json:"quote_id"`
	BaseCurrency    string            `json:"base_currency"`
	Locale          string            `json:"locale"`
	AllowStaleQuote bool              `json:"allow_stale_quote"`
	Items           []LineItemRequest `json:"items"`
}

// LineItemRequest 는 호출자가 문자열 money 형식으로 제공하는 cart line 하나다.
type LineItemRequest struct {
	SKU       string `json:"sku"`
	UnitPrice string `json:"unit_price"`
	Currency  string `json:"currency"`
	Quantity  int    `json:"quantity"`
}

// QuoteResponse 는 안정적인 공개 display-pricing 프로젝션이다.
type QuoteResponse struct {
	QuoteID           string          `json:"quote_id"`
	BaseCurrency      string          `json:"base_currency"`
	DisplayCurrency   string          `json:"display_currency"`
	Subtotal          MoneyValue      `json:"subtotal"`
	DisplayTotal      MoneyValue      `json:"display_total"`
	Items             []LineItemQuote `json:"items"`
	Rate              RateMetadata    `json:"rate"`
	ConversionApplied bool            `json:"conversion_applied"`
}

// LineItemQuote 는 base currency로 가격이 계산된 cart line이다.
type LineItemQuote struct {
	SKU       string     `json:"sku"`
	UnitPrice MoneyValue `json:"unit_price"`
	Quantity  int        `json:"quantity"`
	LineTotal MoneyValue `json:"line_total"`
}

// RateMetadata 는 provider 내부를 누출하지 않고 provider source와 freshness를 노출한다.
type RateMetadata struct {
	Source       string  `json:"source"`
	Rate         string  `json:"rate"`
	ObservedAt   string  `json:"observed_at"`
	FetchedAt    string  `json:"fetched_at"`
	ExpiresAt    string  `json:"expires_at"`
	Stale        bool    `json:"stale"`
	RefreshError *string `json:"refresh_error,omitempty"`
}

// MoneyValue 는 예제에서 사용하는 안정적인 JSON money shape다.
type MoneyValue struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// ErrorResponse 는 안정적인 공개 오류 형식이다.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// NewService 는 exchange-rate pricing service를 만든다.
func NewService(provider money.ExchangeRateProvider, options ...Option) *Service {
	service := &Service{
		provider:         provider,
		operationTimeout: defaultOperationTimeout,
	}
	for _, option := range options {
		option(service)
	}
	if service.operationTimeout <= 0 {
		service.operationTimeout = defaultOperationTimeout
	}
	return service
}

// Quote 는 cart 하나를 base currency로 가격 계산한 뒤 total을 locale currency로 변환한다.
func (s *Service) Quote(ctx context.Context, request QuoteRequest) (QuoteResponse, error) {
	if s == nil {
		return QuoteResponse{}, fmt.Errorf("%w: service is nil", ErrInvalidRequest)
	}
	if s.provider == nil {
		return QuoteResponse{}, ErrProviderUnavailable
	}
	quoteID := strings.TrimSpace(request.QuoteID)
	if quoteID == "" {
		return QuoteResponse{}, fmt.Errorf("%w: quote_id is required", ErrInvalidRequest)
	}
	if len(request.Items) == 0 {
		return QuoteResponse{}, fmt.Errorf("%w: at least one item is required", ErrInvalidRequest)
	}

	baseCurrency, err := parseCurrency(request.BaseCurrency)
	if err != nil {
		return QuoteResponse{}, err
	}
	displayCurrency, err := currencyByLocale(request.Locale)
	if err != nil {
		return QuoteResponse{}, err
	}

	subtotal, items, err := subtotalLines(baseCurrency, request.Items)
	if err != nil {
		return QuoteResponse{}, err
	}

	providerCtx, cancel := s.operationContext(ctx)
	defer cancel()
	displayTotal, quote, err := money.ConvertWithProvider(providerCtx, subtotal, displayCurrency, s.provider)
	if err != nil {
		return QuoteResponse{}, mapProviderError(err)
	}
	if quote.Stale && !request.AllowStaleQuote {
		return QuoteResponse{}, ErrStaleQuote
	}
	displayTotal, err = displayTotal.Round()
	if err != nil {
		return QuoteResponse{}, mapMoneyError(err)
	}

	return QuoteResponse{
		QuoteID:           quoteID,
		BaseCurrency:      baseCurrency.Code(),
		DisplayCurrency:   displayCurrency.Code(),
		Subtotal:          moneyValue(subtotal),
		DisplayTotal:      moneyValue(displayTotal),
		Items:             items,
		Rate:              rateMetadata(quote),
		ConversionApplied: !sameCurrency(baseCurrency, displayCurrency),
	}, nil
}

func (s *Service) operationContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, s.operationTimeout)
}

func subtotalLines(baseCurrency money.Currency, requests []LineItemRequest) (money.Money, []LineItemQuote, error) {
	subtotal, err := roundedZero(baseCurrency)
	if err != nil {
		return money.Money{}, nil, err
	}
	items := make([]LineItemQuote, 0, len(requests))
	for _, item := range requests {
		quote, lineTotal, err := priceLine(baseCurrency, item)
		if err != nil {
			return money.Money{}, nil, err
		}
		subtotal, err = subtotal.Add(lineTotal)
		if err != nil {
			return money.Money{}, nil, mapMoneyError(err)
		}
		items = append(items, quote)
	}
	subtotal, err = subtotal.Round()
	if err != nil {
		return money.Money{}, nil, mapMoneyError(err)
	}
	return subtotal, items, nil
}

func priceLine(baseCurrency money.Currency, item LineItemRequest) (LineItemQuote, money.Money, error) {
	sku := strings.TrimSpace(item.SKU)
	if sku == "" {
		return LineItemQuote{}, money.Money{}, fmt.Errorf("%w: sku is required", ErrInvalidRequest)
	}
	if item.Quantity <= 0 {
		return LineItemQuote{}, money.Money{}, fmt.Errorf("%w: quantity must be positive", ErrInvalidRequest)
	}
	itemCurrency, err := parseCurrency(item.Currency)
	if err != nil {
		return LineItemQuote{}, money.Money{}, err
	}
	if !sameCurrency(itemCurrency, baseCurrency) {
		return LineItemQuote{}, money.Money{}, ErrCurrencyMismatch
	}
	unit, err := money.New(item.UnitPrice, baseCurrency)
	if err != nil {
		return LineItemQuote{}, money.Money{}, mapMoneyError(err)
	}
	lineTotal, err := unit.Mul(strconv.Itoa(item.Quantity))
	if err != nil {
		return LineItemQuote{}, money.Money{}, mapMoneyError(err)
	}
	lineTotal, err = lineTotal.Round()
	if err != nil {
		return LineItemQuote{}, money.Money{}, mapMoneyError(err)
	}
	unit, err = unit.Round()
	if err != nil {
		return LineItemQuote{}, money.Money{}, mapMoneyError(err)
	}
	return LineItemQuote{
		SKU:       sku,
		UnitPrice: moneyValue(unit),
		Quantity:  item.Quantity,
		LineTotal: moneyValue(lineTotal),
	}, lineTotal, nil
}

// NewRouter 는 예제용 HTTP router를 만든다.
func NewRouter(service *Service) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/quotes", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodySize)
		var request QuoteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", "invalid quote request")
			return
		}
		quote, err := service.Quote(c.Request.Context(), request)
		if err != nil {
			writePricingError(c, err)
			return
		}
		c.JSON(http.StatusOK, quote)
	})

	return router
}

func parseCurrency(value string) (money.Currency, error) {
	currency, err := money.ParseCurrency(value)
	if err != nil {
		return money.Currency{}, mapMoneyError(err)
	}
	return currency, nil
}

func currencyByLocale(value string) (money.Currency, error) {
	currency, err := money.CurrencyByLocale(value)
	if err != nil {
		return money.Currency{}, fmt.Errorf("%w: %w", ErrInvalidLocale, err)
	}
	return currency, nil
}

func roundedZero(currency money.Currency) (money.Money, error) {
	zero, err := money.Zero(currency)
	if err != nil {
		return money.Money{}, mapMoneyError(err)
	}
	zero, err = zero.Round()
	if err != nil {
		return money.Money{}, mapMoneyError(err)
	}
	return zero, nil
}

func moneyValue(value money.Money) MoneyValue {
	currency := value.Currency()
	return MoneyValue{
		Amount:   formatAmount(value.Amount(), currency.Scale()),
		Currency: currency.Code(),
	}
}

func rateMetadata(quote money.ExchangeRateQuote) RateMetadata {
	metadata := RateMetadata{
		Source:     quote.Source,
		Rate:       quote.Rate.Rate(),
		ObservedAt: formatTime(quote.ObservedAt),
		FetchedAt:  formatTime(quote.FetchedAt),
		ExpiresAt:  formatTime(quote.ExpiresAt),
		Stale:      quote.Stale,
	}
	if quote.RefreshError != nil {
		message := quote.RefreshError.Error()
		metadata.RefreshError = &message
	}
	return metadata
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatAmount(amount string, scale int) string {
	if scale == 0 {
		return amount
	}
	sign := ""
	if strings.HasPrefix(amount, "-") {
		sign = "-"
		amount = strings.TrimPrefix(amount, "-")
	}
	whole, fractional, ok := strings.Cut(amount, ".")
	if !ok {
		return sign + whole + "." + strings.Repeat("0", scale)
	}
	if len(fractional) >= scale {
		return sign + whole + "." + fractional[:scale]
	}
	return sign + whole + "." + fractional + strings.Repeat("0", scale-len(fractional))
}

func sameCurrency(left, right money.Currency) bool {
	return left.Code() == right.Code()
}

func mapMoneyError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, money.ErrCurrencyMismatch):
		return ErrCurrencyMismatch
	case errors.Is(err, money.ErrInvalidCurrency), errors.Is(err, money.ErrInvalidMoney), errors.Is(err, money.ErrInvalidAmount):
		return fmt.Errorf("%w: invalid money input", ErrInvalidMoney)
	default:
		return err
	}
}

func mapProviderError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, money.ErrCurrencyMismatch):
		return ErrCurrencyMismatch
	case errors.Is(err, money.ErrExchangeRateStale):
		return ErrStaleQuote
	case errors.Is(err, money.ErrExchangeRateProvider), errors.Is(err, money.ErrExchangeRateUnavailable), errors.Is(err, money.ErrUnsupportedExchangeRate):
		return fmt.Errorf("%w: %w", ErrProviderUnavailable, err)
	default:
		return mapMoneyError(err)
	}
}

func writePricingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid quote request")
	case errors.Is(err, ErrInvalidLocale):
		writeError(c, http.StatusBadRequest, "invalid_locale", "locale must include a supported region")
	case errors.Is(err, ErrInvalidMoney):
		writeError(c, http.StatusBadRequest, "invalid_money", "invalid money input")
	case errors.Is(err, ErrCurrencyMismatch):
		writeError(c, http.StatusBadRequest, "currency_mismatch", "line currency must match base currency")
	case errors.Is(err, ErrStaleQuote):
		writeError(c, http.StatusServiceUnavailable, "stale_quote", "exchange-rate quote is stale")
	case errors.Is(err, ErrProviderUnavailable), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeError(c, http.StatusServiceUnavailable, "exchange_rate_unavailable", "exchange-rate provider unavailable")
	default:
		writeError(c, http.StatusInternalServerError, "pricing_error", "pricing failed")
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{ErrorCode: code, Message: message})
}
