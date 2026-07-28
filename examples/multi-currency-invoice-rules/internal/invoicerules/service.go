// Package invoicerules 는 다중 통화 인보이스 규칙 워크숍 예제를 구현한다.
package invoicerules

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/bluetape4k/bluetape-go/money"
	"github.com/gin-gonic/gin"
)

const maxJSONBodySize = 8 << 10

var (
	// ErrInvalidRequest 는 요청 JSON 또는 필드가 유효하지 않음을 나타낸다.
	ErrInvalidRequest = errors.New("invoicerules: invalid request")
	// ErrInvalidMoney 는 통화 또는 십진 금액 입력이 유효하지 않음을 나타낸다.
	ErrInvalidMoney = errors.New("invoicerules: invalid money")
)

// RuleStatus 는 인보이스 규칙 판단의 공개 상태 값이다.
type RuleStatus string

const (
	// RuleAccepted 는 규칙이 적용되어 인보이스에 영향을 주었음을 의미한다.
	RuleAccepted RuleStatus = "accepted"
	// RuleSkipped 는 규칙을 평가했지만 조건과 일치하지 않았음을 의미한다.
	RuleSkipped RuleStatus = "skipped"
)

// Service 는 십진 금액 값을 사용해 인보이스 요청을 평가한다.
type Service struct{}

// InvoiceRequest 는 하나의 인보이스 평가를 위한 HTTP 및 서비스 요청이다.
type InvoiceRequest struct {
	InvoiceID    string            `json:"invoice_id"`
	CustomerTier string            `json:"customer_tier"`
	Region       string            `json:"region"`
	Lines        []LineItemRequest `json:"lines"`
}

// LineItemRequest 는 호출자가 문자열 금액 형태로 제공하는 인보이스 한 줄이다.
type LineItemRequest struct {
	LineID      string `json:"line_id"`
	Description string `json:"description,omitempty"`
	Amount      string `json:"amount"`
	Currency    string `json:"currency"`
	Quantity    int    `json:"quantity"`
	Category    string `json:"category"`
}

// InvoiceResponse 는 안정적으로 공개되는 인보이스 평가 응답 표현이다.
type InvoiceResponse struct {
	InvoiceID         string          `json:"invoice_id"`
	TotalsByCurrency  []CurrencyTotal `json:"totals_by_currency"`
	Lines             []LineItemQuote `json:"lines"`
	Rules             []RuleDecision  `json:"rules"`
	ConversionApplied bool            `json:"conversion_applied"`
}

// CurrencyTotal 은 하나의 통화에 대해 반올림된 합계다.
type CurrencyTotal struct {
	Currency      string     `json:"currency"`
	Subtotal      MoneyValue `json:"subtotal"`
	DiscountTotal MoneyValue `json:"discount_total"`
	TaxTotal      MoneyValue `json:"tax_total"`
	Total         MoneyValue `json:"total"`
}

// LineItemQuote 는 가격이 계산된 인보이스 한 줄이다.
type LineItemQuote struct {
	LineID      string     `json:"line_id"`
	Description string     `json:"description,omitempty"`
	Category    string     `json:"category"`
	UnitAmount  MoneyValue `json:"unit_amount"`
	Quantity    int        `json:"quantity"`
	LineTotal   MoneyValue `json:"line_total"`
}

// RuleDecision 은 한 줄에 대해 하나의 인보이스 규칙이 어떻게 평가됐는지 기록한다.
type RuleDecision struct {
	Name     string      `json:"name"`
	LineID   string      `json:"line_id"`
	Currency string      `json:"currency"`
	Status   RuleStatus  `json:"status"`
	Amount   *MoneyValue `json:"amount,omitempty"`
	Reason   string      `json:"reason,omitempty"`
}

// MoneyValue 는 예제가 공개하는 안정적인 JSON 금액 형태다.
type MoneyValue struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// ErrorResponse 는 안정적으로 공개되는 오류 응답 형태다.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

type currencyAccumulator struct {
	currency money.Currency
	subtotal money.Money
	discount money.Money
	tax      money.Money
}

// NewService 는 인보이스 평가 서비스를 생성한다.
func NewService() *Service {
	return &Service{}
}

// Evaluate 는 결정적인 로컬 인보이스 규칙으로 하나의 인보이스를 평가한다.
func (s *Service) Evaluate(request InvoiceRequest) (InvoiceResponse, error) {
	if s == nil {
		return InvoiceResponse{}, fmt.Errorf("%w: service is nil", ErrInvalidRequest)
	}
	invoiceID := strings.TrimSpace(request.InvoiceID)
	if invoiceID == "" {
		return InvoiceResponse{}, fmt.Errorf("%w: invoice_id is required", ErrInvalidRequest)
	}
	if len(request.Lines) == 0 {
		return InvoiceResponse{}, fmt.Errorf("%w: at least one line is required", ErrInvalidRequest)
	}

	accumulators := make(map[string]*currencyAccumulator)
	lines := make([]LineItemQuote, 0, len(request.Lines))
	rules := make([]RuleDecision, 0, len(request.Lines)*2)
	for _, line := range request.Lines {
		quote, lineTotal, err := priceLine(line)
		if err != nil {
			return InvoiceResponse{}, err
		}
		accumulator, err := accumulatorFor(accumulators, lineTotal.Currency())
		if err != nil {
			return InvoiceResponse{}, err
		}
		accumulator.subtotal, err = accumulator.subtotal.Add(lineTotal)
		if err != nil {
			return InvoiceResponse{}, mapMoneyError(err)
		}

		discount, discountRule, err := applyDiscountRule(lineTotal, quote, request.CustomerTier)
		if err != nil {
			return InvoiceResponse{}, err
		}
		accumulator.discount, err = accumulator.discount.Add(discount)
		if err != nil {
			return InvoiceResponse{}, mapMoneyError(err)
		}
		rules = append(rules, discountRule)

		taxBase, err := lineTotal.Sub(discount)
		if err != nil {
			return InvoiceResponse{}, mapMoneyError(err)
		}
		tax, taxRule, err := applyTaxRule(taxBase, quote, request.Region)
		if err != nil {
			return InvoiceResponse{}, err
		}
		accumulator.tax, err = accumulator.tax.Add(tax)
		if err != nil {
			return InvoiceResponse{}, mapMoneyError(err)
		}
		rules = append(rules, taxRule)

		lines = append(lines, quote)
	}

	totals, err := totalsByCurrency(accumulators)
	if err != nil {
		return InvoiceResponse{}, err
	}
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Currency != rules[j].Currency {
			return rules[i].Currency < rules[j].Currency
		}
		if rules[i].LineID != rules[j].LineID {
			return rules[i].LineID < rules[j].LineID
		}
		return rules[i].Name < rules[j].Name
	})

	return InvoiceResponse{
		InvoiceID:         invoiceID,
		TotalsByCurrency:  totals,
		Lines:             lines,
		Rules:             rules,
		ConversionApplied: false,
	}, nil
}

func priceLine(item LineItemRequest) (LineItemQuote, money.Money, error) {
	lineID := strings.TrimSpace(item.LineID)
	if lineID == "" {
		return LineItemQuote{}, money.Money{}, fmt.Errorf("%w: line_id is required", ErrInvalidRequest)
	}
	if item.Quantity <= 0 {
		return LineItemQuote{}, money.Money{}, fmt.Errorf("%w: quantity must be positive", ErrInvalidRequest)
	}
	category := strings.ToLower(strings.TrimSpace(item.Category))
	if category == "" {
		return LineItemQuote{}, money.Money{}, fmt.Errorf("%w: category is required", ErrInvalidRequest)
	}

	currency, err := parseCurrency(item.Currency)
	if err != nil {
		return LineItemQuote{}, money.Money{}, err
	}
	unit, err := money.New(item.Amount, currency)
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
		LineID:      lineID,
		Description: strings.TrimSpace(item.Description),
		Category:    category,
		UnitAmount:  moneyValue(unit),
		Quantity:    item.Quantity,
		LineTotal:   moneyValue(lineTotal),
	}, lineTotal, nil
}

func accumulatorFor(accumulators map[string]*currencyAccumulator, currency money.Currency) (*currencyAccumulator, error) {
	code := currency.Code()
	if accumulator, ok := accumulators[code]; ok {
		return accumulator, nil
	}
	zero, err := roundedZero(currency)
	if err != nil {
		return nil, err
	}
	accumulator := &currencyAccumulator{
		currency: currency,
		subtotal: zero,
		discount: zero,
		tax:      zero,
	}
	accumulators[code] = accumulator
	return accumulator, nil
}

func applyDiscountRule(lineTotal money.Money, line LineItemQuote, customerTier string) (money.Money, RuleDecision, error) {
	zero, err := roundedZero(lineTotal.Currency())
	if err != nil {
		return money.Money{}, RuleDecision{}, err
	}
	rule := RuleDecision{
		Name:     "vip-service-discount",
		LineID:   line.LineID,
		Currency: line.LineTotal.Currency,
		Status:   RuleSkipped,
	}
	if !strings.EqualFold(strings.TrimSpace(customerTier), "vip") {
		rule.Reason = "tier_not_eligible"
		return zero, rule, nil
	}
	if line.Category != "service" {
		rule.Reason = "category_not_service"
		return zero, rule, nil
	}
	discount, err := lineTotal.Mul("0.05")
	if err != nil {
		return money.Money{}, RuleDecision{}, mapMoneyError(err)
	}
	discount, err = discount.Round()
	if err != nil {
		return money.Money{}, RuleDecision{}, mapMoneyError(err)
	}
	amount := moneyValue(discount)
	rule.Status = RuleAccepted
	rule.Amount = &amount
	return discount, rule, nil
}

func applyTaxRule(taxBase money.Money, line LineItemQuote, region string) (money.Money, RuleDecision, error) {
	zero, err := roundedZero(taxBase.Currency())
	if err != nil {
		return money.Money{}, RuleDecision{}, err
	}
	rule := RuleDecision{
		Name:     "regional-vat",
		LineID:   line.LineID,
		Currency: line.LineTotal.Currency,
		Status:   RuleSkipped,
	}
	if !strings.EqualFold(strings.TrimSpace(region), "EU") {
		rule.Reason = "region_not_eu"
		return zero, rule, nil
	}
	if line.Category == "tax_exempt" {
		rule.Reason = "category_tax_exempt"
		return zero, rule, nil
	}
	tax, err := taxBase.Mul("0.20")
	if err != nil {
		return money.Money{}, RuleDecision{}, mapMoneyError(err)
	}
	tax, err = tax.Round()
	if err != nil {
		return money.Money{}, RuleDecision{}, mapMoneyError(err)
	}
	amount := moneyValue(tax)
	rule.Status = RuleAccepted
	rule.Amount = &amount
	return tax, rule, nil
}

func totalsByCurrency(accumulators map[string]*currencyAccumulator) ([]CurrencyTotal, error) {
	codes := make([]string, 0, len(accumulators))
	for code := range accumulators {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	totals := make([]CurrencyTotal, 0, len(codes))
	for _, code := range codes {
		accumulator := accumulators[code]
		subtotal, err := accumulator.subtotal.Round()
		if err != nil {
			return nil, mapMoneyError(err)
		}
		discount, err := accumulator.discount.Round()
		if err != nil {
			return nil, mapMoneyError(err)
		}
		tax, err := accumulator.tax.Round()
		if err != nil {
			return nil, mapMoneyError(err)
		}
		total, err := subtotal.Sub(discount)
		if err != nil {
			return nil, mapMoneyError(err)
		}
		total, err = total.Add(tax)
		if err != nil {
			return nil, mapMoneyError(err)
		}
		total, err = total.Round()
		if err != nil {
			return nil, mapMoneyError(err)
		}
		totals = append(totals, CurrencyTotal{
			Currency:      accumulator.currency.Code(),
			Subtotal:      moneyValue(subtotal),
			DiscountTotal: moneyValue(discount),
			TaxTotal:      moneyValue(tax),
			Total:         moneyValue(total),
		})
	}
	return totals, nil
}

// NewRouter 는 예제용 HTTP 라우터를 생성한다.
func NewRouter(service *Service) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	_ = router.SetTrustedProxies(nil)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/invoices/evaluate", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodySize)
		var request InvoiceRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_request", "invalid invoice request")
			return
		}
		invoice, err := service.Evaluate(request)
		if err != nil {
			writeInvoiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, invoice)
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

func roundedZero(currency money.Currency) (money.Money, error) {
	zero, err := money.Zero(currency)
	if err != nil {
		return money.Money{}, mapMoneyError(err)
	}
	rounded, err := zero.Round()
	if err != nil {
		return money.Money{}, mapMoneyError(err)
	}
	return rounded, nil
}

func moneyValue(value money.Money) MoneyValue {
	currency := value.Currency()
	return MoneyValue{
		Amount:   formatAmount(value.Amount(), currency.Scale()),
		Currency: currency.Code(),
	}
}

func formatAmount(amount string, scale int) string {
	if scale == 0 {
		whole, _, _ := strings.Cut(amount, ".")
		return whole
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

func mapMoneyError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, money.ErrInvalidCurrency), errors.Is(err, money.ErrInvalidMoney), errors.Is(err, money.ErrInvalidAmount):
		return fmt.Errorf("%w: invalid money input", ErrInvalidMoney)
	default:
		return err
	}
}

func writeInvoiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid invoice request")
	case errors.Is(err, ErrInvalidMoney):
		writeError(c, http.StatusBadRequest, "invalid_money", "invalid money input")
	default:
		writeError(c, http.StatusInternalServerError, "invoice_error", "invoice evaluation failed")
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{ErrorCode: code, Message: message})
}
