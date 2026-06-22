// Package moneypricing implements the money and rule based pricing workshop example.
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
	// ErrInvalidRequest reports invalid request JSON or fields.
	ErrInvalidRequest = errors.New("moneypricing: invalid request")
	// ErrInvalidMoney reports invalid currency or decimal money input.
	ErrInvalidMoney = errors.New("moneypricing: invalid money")
	// ErrCurrencyMismatch reports mixed currencies inside one cart.
	ErrCurrencyMismatch = errors.New("moneypricing: currency mismatch")
)

// RuleStatus is the public status for a pricing rule decision.
type RuleStatus string

const (
	// RuleAccepted means the rule applied and affected the quote.
	RuleAccepted RuleStatus = "accepted"
	// RuleRejected means the rule was evaluated but could not apply.
	RuleRejected RuleStatus = "rejected"
	// RuleSkipped means the rule was evaluated and did not match.
	RuleSkipped RuleStatus = "skipped"
)

// Service prices cart quote requests with decimal-backed money values.
type Service struct{}

// QuoteRequest is the HTTP and service request for one cart quote.
type QuoteRequest struct {
	CartID       string            `json:"cart_id"`
	Currency     string            `json:"currency"`
	CustomerTier string            `json:"customer_tier"`
	CouponCode   string            `json:"coupon_code"`
	Items        []LineItemRequest `json:"items"`
}

// LineItemRequest is one cart line in caller-provided string money form.
type LineItemRequest struct {
	SKU       string `json:"sku"`
	UnitPrice string `json:"unit_price"`
	Currency  string `json:"currency"`
	Quantity  int    `json:"quantity"`
}

// QuoteResponse is the stable public pricing projection.
type QuoteResponse struct {
	CartID        string          `json:"cart_id"`
	Currency      string          `json:"currency"`
	Subtotal      MoneyValue      `json:"subtotal"`
	DiscountTotal MoneyValue      `json:"discount_total"`
	Total         MoneyValue      `json:"total"`
	Items         []LineItemQuote `json:"items"`
	Rules         []RuleDecision  `json:"rules"`
}

// LineItemQuote is a priced cart line.
type LineItemQuote struct {
	SKU       string     `json:"sku"`
	UnitPrice MoneyValue `json:"unit_price"`
	Quantity  int        `json:"quantity"`
	LineTotal MoneyValue `json:"line_total"`
}

// RuleDecision records how one pricing rule evaluated.
type RuleDecision struct {
	Name   string      `json:"name"`
	Status RuleStatus  `json:"status"`
	Amount *MoneyValue `json:"amount,omitempty"`
	Reason string      `json:"reason,omitempty"`
}

// MoneyValue is the example's stable JSON money shape.
type MoneyValue struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// ErrorResponse is the stable public error shape.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// NewService creates a pricing service.
func NewService() *Service {
	return &Service{}
}

// Quote prices one cart with deterministic local pricing rules.
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

// NewRouter creates the HTTP router for the example.
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
