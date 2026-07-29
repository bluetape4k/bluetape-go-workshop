// Package moneypricing 은 금액과 규칙 기반 가격 산정 워크숍 예제를 구현한다.
package moneypricing

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bluetape4k/bluetape-go/money"
	"github.com/gin-gonic/gin"
)

const (
	maxJSONBodySize = 8 << 10
)

var (
	// ErrInvalidRequest 는 요청 JSON 또는 필드가 유효하지 않음을 나타낸다.
	ErrInvalidRequest = errors.New("moneypricing: invalid request")
	// ErrInvalidMoney 는 통화 또는 십진 금액 입력이 유효하지 않음을 나타낸다.
	ErrInvalidMoney = errors.New("moneypricing: invalid money")
	// ErrCurrencyMismatch 는 하나의 장바구니에 서로 다른 통화가 섞였음을 나타낸다.
	ErrCurrencyMismatch = errors.New("moneypricing: currency mismatch")
)

// RuleStatus 는 가격 산정 규칙 판단의 공개 상태 값이다.
type RuleStatus string

const (
	// RuleAccepted 는 규칙이 적용되어 견적에 영향을 주었음을 의미한다.
	RuleAccepted RuleStatus = "accepted"
	// RuleRejected 는 규칙을 평가했지만 적용할 수 없었음을 의미한다.
	RuleRejected RuleStatus = "rejected"
	// RuleSkipped 는 규칙을 평가했지만 조건과 일치하지 않았음을 의미한다.
	RuleSkipped RuleStatus = "skipped"
)

// Service 는 십진 금액 값을 사용해 장바구니 견적 요청의 가격을 산정한다.
type Service struct{}

// QuoteRequest 는 하나의 장바구니 견적을 위한 HTTP 및 서비스 요청이다.
type QuoteRequest struct {
	CartID       string            `json:"cart_id"`
	Currency     string            `json:"currency"`
	CustomerTier string            `json:"customer_tier"`
	CouponCode   string            `json:"coupon_code"`
	Items        []LineItemRequest `json:"items"`
}

// LineItemRequest 는 호출자가 문자열 금액 형태로 제공하는 장바구니 한 줄이다.
type LineItemRequest struct {
	SKU       string `json:"sku"`
	UnitPrice string `json:"unit_price"`
	Currency  string `json:"currency"`
	Quantity  int    `json:"quantity"`
}

// QuoteResponse 는 안정적으로 공개되는 가격 산정 응답 표현이다.
type QuoteResponse struct {
	CartID        string          `json:"cart_id"`
	Currency      string          `json:"currency"`
	Subtotal      MoneyValue      `json:"subtotal"`
	DiscountTotal MoneyValue      `json:"discount_total"`
	Total         MoneyValue      `json:"total"`
	Items         []LineItemQuote `json:"items"`
	Rules         []RuleDecision  `json:"rules"`
}

// LineItemQuote 는 가격이 계산된 장바구니 한 줄이다.
type LineItemQuote struct {
	SKU       string     `json:"sku"`
	UnitPrice MoneyValue `json:"unit_price"`
	Quantity  int        `json:"quantity"`
	LineTotal MoneyValue `json:"line_total"`
}

// RuleDecision 은 하나의 가격 산정 규칙이 어떻게 평가됐는지 기록한다.
type RuleDecision struct {
	Name   string      `json:"name"`
	Status RuleStatus  `json:"status"`
	Amount *MoneyValue `json:"amount,omitempty"`
	Reason string      `json:"reason,omitempty"`
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

// NewService 는 가격 산정 서비스를 생성한다.
func NewService() *Service {
	return &Service{}
}

// Quote 는 결정적인 로컬 가격 산정 규칙으로 하나의 장바구니 가격을 계산한다.
func (s *Service) Quote(request QuoteRequest) (QuoteResponse, error) {
	if s == nil {
		return QuoteResponse{}, fmt.Errorf("%w: service is nil", ErrInvalidRequest)
	}
	cartID := strings.TrimSpace(request.CartID)
	if cartID == "" {
		return QuoteResponse{}, fmt.Errorf("%w: cart_id is required", ErrInvalidRequest)
	}
	if len(request.Items) == 0 {
		return QuoteResponse{}, fmt.Errorf("%w: at least one item is required", ErrInvalidRequest)
	}

	currency, err := parseCurrency(request.Currency)
	if err != nil {
		return QuoteResponse{}, err
	}
	subtotal, err := money.Zero(currency)
	if err != nil {
		return QuoteResponse{}, mapMoneyError(err)
	}

	items := make([]LineItemQuote, 0, len(request.Items))
	for _, item := range request.Items {
		quote, err := priceLine(currency, item)
		if err != nil {
			return QuoteResponse{}, err
		}
		lineTotal, err := money.New(quote.LineTotal.Amount, currency)
		if err != nil {
			return QuoteResponse{}, mapMoneyError(err)
		}
		subtotal, err = subtotal.Add(lineTotal)
		if err != nil {
			return QuoteResponse{}, mapMoneyError(err)
		}
		items = append(items, quote)
	}
	subtotal, err = subtotal.Round()
	if err != nil {
		return QuoteResponse{}, mapMoneyError(err)
	}

	discountTotal, rules, err := applyRules(currency, subtotal, request.CustomerTier, request.CouponCode)
	if err != nil {
		return QuoteResponse{}, err
	}
	total, err := subtotal.Sub(discountTotal)
	if err != nil {
		return QuoteResponse{}, mapMoneyError(err)
	}
	total, err = total.Round()
	if err != nil {
		return QuoteResponse{}, mapMoneyError(err)
	}

	return QuoteResponse{
		CartID:        cartID,
		Currency:      currency.Code(),
		Subtotal:      moneyValue(subtotal),
		DiscountTotal: moneyValue(discountTotal),
		Total:         moneyValue(total),
		Items:         items,
		Rules:         rules,
	}, nil
}

func priceLine(cartCurrency money.Currency, item LineItemRequest) (LineItemQuote, error) {
	sku := strings.TrimSpace(item.SKU)
	if sku == "" {
		return LineItemQuote{}, fmt.Errorf("%w: sku is required", ErrInvalidRequest)
	}
	if item.Quantity <= 0 {
		return LineItemQuote{}, fmt.Errorf("%w: quantity must be positive", ErrInvalidRequest)
	}
	itemCurrency, err := parseCurrency(item.Currency)
	if err != nil {
		return LineItemQuote{}, err
	}
	if itemCurrency.Code() != cartCurrency.Code() {
		return LineItemQuote{}, ErrCurrencyMismatch
	}
	unit, err := money.New(item.UnitPrice, cartCurrency)
	if err != nil {
		return LineItemQuote{}, mapMoneyError(err)
	}
	lineTotal, err := unit.Mul(strconv.Itoa(item.Quantity))
	if err != nil {
		return LineItemQuote{}, mapMoneyError(err)
	}
	lineTotal, err = lineTotal.Round()
	if err != nil {
		return LineItemQuote{}, mapMoneyError(err)
	}
	unit, err = unit.Round()
	if err != nil {
		return LineItemQuote{}, mapMoneyError(err)
	}
	return LineItemQuote{
		SKU:       sku,
		UnitPrice: moneyValue(unit),
		Quantity:  item.Quantity,
		LineTotal: moneyValue(lineTotal),
	}, nil
}

func applyRules(currency money.Currency, subtotal money.Money, customerTier, couponCode string) (money.Money, []RuleDecision, error) {
	discountTotal, err := money.Zero(currency)
	if err != nil {
		return money.Money{}, nil, mapMoneyError(err)
	}
	discountTotal, err = discountTotal.Round()
	if err != nil {
		return money.Money{}, nil, mapMoneyError(err)
	}

	rules := make([]RuleDecision, 0, 2)
	if strings.EqualFold(strings.TrimSpace(customerTier), "vip") {
		vipDiscount, err := subtotal.Mul("0.10")
		if err != nil {
			return money.Money{}, nil, mapMoneyError(err)
		}
		vipDiscount, err = vipDiscount.Round()
		if err != nil {
			return money.Money{}, nil, mapMoneyError(err)
		}
		discountTotal, err = discountTotal.Add(vipDiscount)
		if err != nil {
			return money.Money{}, nil, mapMoneyError(err)
		}
		amount := moneyValue(vipDiscount)
		rules = append(rules, RuleDecision{Name: "vip-ten-percent", Status: RuleAccepted, Amount: &amount})
	} else if strings.TrimSpace(customerTier) != "" {
		rules = append(rules, RuleDecision{Name: "vip-ten-percent", Status: RuleSkipped, Reason: "tier_not_eligible"})
	}

	coupon := strings.ToUpper(strings.TrimSpace(couponCode))
	switch coupon {
	case "":
	case "SAVE10":
		fixedDiscount, err := money.New("10.00", currency)
		if err != nil {
			return money.Money{}, nil, mapMoneyError(err)
		}
		remainingSubtotal, err := subtotal.Sub(discountTotal)
		if err != nil {
			return money.Money{}, nil, mapMoneyError(err)
		}
		cmp, err := fixedDiscount.Cmp(remainingSubtotal)
		if err != nil {
			return money.Money{}, nil, mapMoneyError(err)
		}
		if cmp > 0 {
			rules = append(rules, RuleDecision{Name: "coupon-save10", Status: RuleRejected, Reason: "discount_exceeds_subtotal"})
			return discountTotal, rules, nil
		}
		discountTotal, err = discountTotal.Add(fixedDiscount)
		if err != nil {
			return money.Money{}, nil, mapMoneyError(err)
		}
		discountTotal, err = discountTotal.Round()
		if err != nil {
			return money.Money{}, nil, mapMoneyError(err)
		}
		amount := moneyValue(fixedDiscount)
		rules = append(rules, RuleDecision{Name: "coupon-save10", Status: RuleAccepted, Amount: &amount})
	default:
		rules = append(rules, RuleDecision{Name: "coupon", Status: RuleRejected, Reason: "unsupported_coupon"})
	}

	return discountTotal, rules, nil
}

// NewRouter 는 예제용 HTTP 라우터를 생성한다.
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
		quote, err := service.Quote(request)
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

func moneyValue(value money.Money) MoneyValue {
	currency := value.Currency()
	return MoneyValue{
		Amount:   formatAmount(value.Amount(), currency.Scale()),
		Currency: currency.Code(),
	}
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

func writePricingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid quote request")
	case errors.Is(err, ErrInvalidMoney):
		writeError(c, http.StatusBadRequest, "invalid_money", "invalid money input")
	case errors.Is(err, ErrCurrencyMismatch):
		writeError(c, http.StatusBadRequest, "currency_mismatch", "cart contains mixed currencies")
	default:
		writeError(c, http.StatusInternalServerError, "pricing_error", "pricing failed")
	}
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{ErrorCode: code, Message: message})
}
