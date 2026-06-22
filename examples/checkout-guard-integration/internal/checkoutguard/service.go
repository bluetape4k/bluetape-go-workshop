// Package checkoutguard implements the utility-composed checkout guard example.
package checkoutguard

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/id"
	btjwt "github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/money"
	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/gin-gonic/gin"
)

const (
	// DefaultIssuer is the demo issuer expected by the checkout guard.
	DefaultIssuer = "checkout-guard-integration"
	// AccessAudience is the audience required for checkout access tokens.
	AccessAudience = "checkout-api"
	// TokenUseAccess marks tokens accepted by the checkout guard.
	TokenUseAccess = "access"
	// RequiredScope is the scope required by guarded checkout submissions.
	RequiredScope = "checkout:submit"

	defaultRole        = "customer"
	defaultTokenTTL    = 5 * time.Minute
	maxJSONBodySize    = 8 << 10
	defaultExpectedN   = 1_000
	defaultTargetFPP   = 0.01
	defaultScenario    = "checkout-submission-guard"
	restrictedCategory = "restricted"
)

var (
	// ErrMissingToken reports a missing bearer token.
	ErrMissingToken = errors.New("checkoutguard: missing token")
	// ErrInvalidToken reports a malformed or unverifiable bearer token.
	ErrInvalidToken = errors.New("checkoutguard: invalid token")
	// ErrExpiredToken reports a verified token that is past expiration.
	ErrExpiredToken = errors.New("checkoutguard: expired token")
	// ErrInvalidClaims reports a verified token with the wrong claim contract.
	ErrInvalidClaims = errors.New("checkoutguard: invalid claims")
	// ErrInvalidRequest reports invalid request JSON or fields.
	ErrInvalidRequest = errors.New("checkoutguard: invalid request")
	// ErrInvalidMoney reports invalid money values or currency mismatches.
	ErrInvalidMoney = errors.New("checkoutguard: invalid money")
	// ErrRuleDenied reports a local checkout eligibility rule denial.
	ErrRuleDenied = errors.New("checkoutguard: rule denied")
	// ErrDuplicateSubmission reports a repeated or probably repeated idempotency key.
	ErrDuplicateSubmission = errors.New("checkoutguard: duplicate submission")
	// ErrGuard reports unexpected guard setup or runtime failures.
	ErrGuard = errors.New("checkoutguard: guard error")
)

type stringGenerator interface {
	NextString() (string, error)
}

// Decision is the public admission decision.
type Decision string

const (
	// DecisionAdmit means the idempotency key was definitely not present before insertion.
	DecisionAdmit Decision = "admit"
	// DecisionProbablySeen means the idempotency key might have been seen before.
	DecisionProbablySeen Decision = "probably_seen"
)

const (
	// ReasonDefinitelyNew describes the no-false-negative Bloom filter path.
	ReasonDefinitelyNew = "definitely_new"
	// ReasonMightBeDuplicate describes the possible duplicate or false-positive path.
	ReasonMightBeDuplicate = "might_be_duplicate_or_false_positive"
)

// RuleStatus is the public status for local checkout rules.
type RuleStatus string

const (
	// RuleAccepted means a rule applied and changed or approved the checkout.
	RuleAccepted RuleStatus = "accepted"
	// RuleSkipped means a rule did not apply.
	RuleSkipped RuleStatus = "skipped"
	// RuleDenied means a rule blocked the checkout.
	RuleDenied RuleStatus = "denied"
)

// Config holds service wiring for the demo guard.
type Config struct {
	issuer                   string
	accessAudience           string
	requiredRole             string
	requiredScope            string
	secret                   []byte
	clock                    func() time.Time
	idGenerator              stringGenerator
	expectedInsertions       uint64
	falsePositiveProbability float64
	scenarioName             string
}

// Option customizes Config before the service is constructed.
type Option func(*Config)

// WithClock sets the service clock used for token issue and parse checks.
func WithClock(clock func() time.Time) Option {
	return func(cfg *Config) {
		if clock != nil {
			cfg.clock = clock
		}
	}
}

// WithSecret sets the fixed HMAC demo secret.
func WithSecret(secret []byte) Option {
	return func(cfg *Config) {
		cfg.secret = append([]byte(nil), secret...)
	}
}

// WithIDGenerator sets the ID generator used for sessions, JWT IDs, requests, and orders.
func WithIDGenerator(generator stringGenerator) Option {
	return func(cfg *Config) {
		if generator != nil {
			cfg.idGenerator = generator
		}
	}
}

// WithBloomConfig sets the expected Bloom filter size and target false-positive probability.
func WithBloomConfig(expectedInsertions uint64, falsePositiveProbability float64) Option {
	return func(cfg *Config) {
		if expectedInsertions > 0 {
			cfg.expectedInsertions = expectedInsertions
		}
		if falsePositiveProbability > 0 {
			cfg.falsePositiveProbability = falsePositiveProbability
		}
	}
}

// Service owns token issue, token verification, money rules, and Bloom admission.
type Service struct {
	provider       *btjwt.Provider
	idGenerator    stringGenerator
	clock          func() time.Time
	issuer         string
	accessAudience string
	requiredRole   string
	requiredScope  string
	scenarioName   string
	mu             sync.Mutex
	filter         probabilistic.BloomFilter[string]
}

// TokenRequest is the local demo access-token issue request.
type TokenRequest struct {
	Subject    string   `json:"subject"`
	Role       string   `json:"role"`
	Scopes     []string `json:"scopes"`
	TTLSeconds int      `json:"ttl_seconds"`
}

// TokenResponse is the local demo access-token issue response.
type TokenResponse struct {
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	Token            string `json:"token"`
	SessionID        string `json:"session_id"`
}

// CheckoutRequest is the protected checkout submission request.
type CheckoutRequest struct {
	CheckoutID     string            `json:"checkout_id"`
	IdempotencyKey string            `json:"idempotency_key"`
	CustomerTier   string            `json:"customer_tier"`
	Region         string            `json:"region"`
	Currency       string            `json:"currency"`
	Items          []LineItemRequest `json:"items"`
}

// LineItemRequest is one checkout line.
type LineItemRequest struct {
	LineID    string `json:"line_id"`
	SKU       string `json:"sku"`
	UnitPrice string `json:"unit_price"`
	Currency  string `json:"currency"`
	Quantity  int    `json:"quantity"`
	Category  string `json:"category"`
}

// CheckoutResponse is the accepted checkout projection returned to callers.
type CheckoutResponse struct {
	CheckoutID string            `json:"checkout_id"`
	RequestID  string            `json:"request_id"`
	OrderID    string            `json:"order_id"`
	Subject    string            `json:"subject"`
	SessionID  string            `json:"session_id"`
	Admission  AdmissionDecision `json:"admission"`
	Pricing    PricingSummary    `json:"pricing"`
	Rules      []RuleDecision    `json:"rules"`
}

// AdmissionDecision is the stable public Bloom decision projection.
type AdmissionDecision struct {
	IdempotencyKey string      `json:"idempotency_key"`
	Scenario       string      `json:"scenario"`
	Decision       Decision    `json:"decision"`
	Reason         string      `json:"reason"`
	Accepted       bool        `json:"accepted"`
	Stats          FilterStats `json:"stats"`
}

// FilterStats reports approximate Bloom filter state.
type FilterStats struct {
	ExpectedInsertions                 uint64  `json:"expected_insertions"`
	TargetFalsePositiveProbability     float64 `json:"target_false_positive_probability"`
	ApproximateElementCount            uint64  `json:"approximate_element_count"`
	ExpectedFalsePositiveProbability   float64 `json:"expected_false_positive_probability"`
	BitSize                            uint64  `json:"bit_size"`
	HashFunctionCount                  uint64  `json:"hash_function_count"`
	DurableAuthoritativeStoreRequired  bool    `json:"durable_authoritative_store_required"`
	ProbabilisticFalsePositivePossible bool    `json:"probabilistic_false_positive_possible"`
}

// PricingSummary reports rounded money totals for one checkout currency.
type PricingSummary struct {
	Currency      string      `json:"currency"`
	Subtotal      MoneyValue  `json:"subtotal"`
	DiscountTotal MoneyValue  `json:"discount_total"`
	TaxTotal      MoneyValue  `json:"tax_total"`
	Total         MoneyValue  `json:"total"`
	Lines         []LineQuote `json:"lines"`
}

// LineQuote reports rounded line totals.
type LineQuote struct {
	LineID    string     `json:"line_id"`
	SKU       string     `json:"sku"`
	Category  string     `json:"category"`
	Quantity  int        `json:"quantity"`
	LineTotal MoneyValue `json:"line_total"`
}

// RuleDecision is a stable public local-rule projection.
type RuleDecision struct {
	Name     string      `json:"name"`
	LineID   string      `json:"line_id,omitempty"`
	Status   RuleStatus  `json:"status"`
	Currency string      `json:"currency,omitempty"`
	Amount   *MoneyValue `json:"amount,omitempty"`
	Reason   string      `json:"reason,omitempty"`
}

// MoneyValue is the JSON-safe money representation.
type MoneyValue struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// ErrorResponse is the stable public error shape.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

type tokenClaims struct {
	subject   string
	role      string
	scope     string
	sessionID string
}

// NewService builds the demo service with deterministic local JWT and Bloom defaults.
func NewService(options ...Option) (*Service, error) {
	cfg, err := defaultConfig()
	if err != nil {
		return nil, err
	}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	if len(cfg.secret) < 32 {
		return nil, fmt.Errorf("%w: HMAC secret must be at least 32 bytes", ErrInvalidRequest)
	}
	provider, err := btjwt.NewFixedHMACProvider(
		btjwt.HS256,
		cfg.secret,
		btjwt.WithClock(cfg.clock),
		btjwt.WithKeyIDGenerator(func() (string, error) { return "local-demo-key", nil }),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: create jwt provider: %w", ErrInvalidToken, err)
	}
	bloomConfig, err := probabilistic.NewConfig(cfg.expectedInsertions, cfg.falsePositiveProbability)
	if err != nil {
		return nil, fmt.Errorf("%w: create bloom config: %w", ErrGuard, err)
	}
	filter, err := probabilistic.NewStringBloomFilter(bloomConfig)
	if err != nil {
		return nil, fmt.Errorf("%w: create bloom filter: %w", ErrGuard, err)
	}
	return &Service{
		provider:       provider,
		idGenerator:    cfg.idGenerator,
		clock:          cfg.clock,
		issuer:         cfg.issuer,
		accessAudience: cfg.accessAudience,
		requiredRole:   cfg.requiredRole,
		requiredScope:  cfg.requiredScope,
		scenarioName:   cfg.scenarioName,
		filter:         filter,
	}, nil
}

// IssueToken issues a local demo access token for the checkout guard.
func (s *Service) IssueToken(request TokenRequest) (TokenResponse, error) {
	if err := validateTokenRequest(request); err != nil {
		return TokenResponse{}, err
	}
	sessionID, err := s.nextID()
	if err != nil {
		return TokenResponse{}, err
	}
	jti, err := s.nextID()
	if err != nil {
		return TokenResponse{}, err
	}
	scopes := strings.Join(normalizeScopes(request.Scopes), " ")
	ttl := time.Duration(request.TTLSeconds) * time.Second
	token, err := s.provider.Compose(
		btjwt.WithIssuer(s.issuer),
		btjwt.WithSubject(strings.TrimSpace(request.Subject)),
		btjwt.WithAudience(s.accessAudience),
		btjwt.WithExpiresAfter(ttl),
		btjwt.WithJWTID(jti),
		btjwt.WithClaim("token_use", TokenUseAccess),
		btjwt.WithClaim("role", strings.TrimSpace(request.Role)),
		btjwt.WithClaim("scope", scopes),
		btjwt.WithClaim("session_id", sessionID),
	)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("%w: compose access token", ErrInvalidToken)
	}
	return TokenResponse{
		TokenType:        "Bearer",
		ExpiresInSeconds: request.TTLSeconds,
		Token:            token,
		SessionID:        sessionID,
	}, nil
}

// GuardCheckout verifies claims, prices the checkout, and admits the idempotency key.
func (s *Service) GuardCheckout(authorization string, request CheckoutRequest) (CheckoutResponse, error) {
	claims, err := s.verifyAuthorization(authorization)
	if err != nil {
		return CheckoutResponse{}, err
	}
	normalized, currency, err := normalizeCheckoutRequest(request)
	if err != nil {
		return CheckoutResponse{}, err
	}
	pricing, rules, err := priceCheckout(normalized, currency)
	if err != nil {
		return CheckoutResponse{}, err
	}
	admission, err := s.admit(normalized.IdempotencyKey)
	if err != nil {
		return CheckoutResponse{}, err
	}
	requestID, err := s.nextID()
	if err != nil {
		return CheckoutResponse{}, err
	}
	orderID, err := s.nextID()
	if err != nil {
		return CheckoutResponse{}, err
	}
	return CheckoutResponse{
		CheckoutID: normalized.CheckoutID,
		RequestID:  requestID,
		OrderID:    orderID,
		Subject:    claims.subject,
		SessionID:  claims.sessionID,
		Admission:  admission,
		Pricing:    pricing,
		Rules:      rules,
	}, nil
}

func (s *Service) verifyAuthorization(authorization string) (tokenClaims, error) {
	token, err := bearerToken(authorization)
	if err != nil {
		return tokenClaims{}, err
	}
	reader, err := s.provider.Parse(
		token,
		btjwt.WithExpectedIssuer(s.issuer),
		btjwt.WithExpectedAudience(s.accessAudience),
		btjwt.WithExpirationRequired(),
		btjwt.WithParseClock(s.clock),
	)
	if err != nil {
		if errors.Is(err, btjwt.ErrExpiredToken) {
			return tokenClaims{}, fmt.Errorf("%w: token expired", ErrExpiredToken)
		}
		return tokenClaims{}, fmt.Errorf("%w: token could not be verified", ErrInvalidToken)
	}
	tokenUse, _ := reader.ClaimString("token_use")
	role, _ := reader.ClaimString("role")
	scope, _ := reader.ClaimString("scope")
	sessionID, _ := reader.ClaimString("session_id")
	claims := tokenClaims{
		subject:   reader.Subject(),
		role:      role,
		scope:     scope,
		sessionID: sessionID,
	}
	if claims.subject == "" ||
		claims.sessionID == "" ||
		tokenUse != TokenUseAccess ||
		claims.role != s.requiredRole ||
		!hasScope(claims.scope, s.requiredScope) {
		return tokenClaims{}, fmt.Errorf("%w: access token claim contract mismatch", ErrInvalidClaims)
	}
	return claims, nil
}

func (s *Service) admit(idempotencyKey string) (AdmissionDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.filter.MightContain(idempotencyKey) {
		return AdmissionDecision{}, fmt.Errorf("%w: idempotency key probably seen", ErrDuplicateSubmission)
	}
	s.filter.Put(idempotencyKey)
	return AdmissionDecision{
		IdempotencyKey: idempotencyKey,
		Scenario:       s.scenarioName,
		Decision:       DecisionAdmit,
		Reason:         ReasonDefinitelyNew,
		Accepted:       true,
		Stats:          s.statsLocked(),
	}, nil
}

func (s *Service) statsLocked() FilterStats {
	return FilterStats{
		ExpectedInsertions:                 s.filter.ExpectedInsertions(),
		TargetFalsePositiveProbability:     s.filter.FalsePositiveProbability(),
		ApproximateElementCount:            s.filter.ApproximateElementCount(),
		ExpectedFalsePositiveProbability:   s.filter.ExpectedFPP(),
		BitSize:                            s.filter.BitSize(),
		HashFunctionCount:                  s.filter.HashFunctionCount(),
		DurableAuthoritativeStoreRequired:  true,
		ProbabilisticFalsePositivePossible: true,
	}
}

func priceCheckout(request CheckoutRequest, currency money.Currency) (PricingSummary, []RuleDecision, error) {
	subtotal, err := roundedZero(currency)
	if err != nil {
		return PricingSummary{}, nil, err
	}
	discountTotal, err := roundedZero(currency)
	if err != nil {
		return PricingSummary{}, nil, err
	}
	taxTotal, err := roundedZero(currency)
	if err != nil {
		return PricingSummary{}, nil, err
	}
	lines := make([]LineQuote, 0, len(request.Items))
	rules := make([]RuleDecision, 0, len(request.Items)*3)

	for _, item := range request.Items {
		if strings.EqualFold(item.Category, restrictedCategory) {
			rules = append(rules, RuleDecision{
				Name:   "restricted-category",
				LineID: item.LineID,
				Status: RuleDenied,
				Reason: "category_restricted",
			})
			return PricingSummary{}, rules, fmt.Errorf("%w: restricted item category", ErrRuleDenied)
		}
		unitPrice, err := money.New(item.UnitPrice, currency)
		if err != nil {
			return PricingSummary{}, nil, mapMoneyError(err)
		}
		lineTotal, err := unitPrice.Mul(strconv.Itoa(item.Quantity))
		if err != nil {
			return PricingSummary{}, nil, mapMoneyError(err)
		}
		lineTotal, err = lineTotal.Round()
		if err != nil {
			return PricingSummary{}, nil, mapMoneyError(err)
		}
		subtotal, err = subtotal.Add(lineTotal)
		if err != nil {
			return PricingSummary{}, nil, mapMoneyError(err)
		}
		lines = append(lines, LineQuote{
			LineID:    item.LineID,
			SKU:       item.SKU,
			Category:  item.Category,
			Quantity:  item.Quantity,
			LineTotal: moneyValue(lineTotal),
		})

		discount, discountRule, err := applyDiscountRule(lineTotal, item, request.CustomerTier)
		if err != nil {
			return PricingSummary{}, nil, err
		}
		rules = append(rules, discountRule)
		discountTotal, err = discountTotal.Add(discount)
		if err != nil {
			return PricingSummary{}, nil, mapMoneyError(err)
		}

		taxBase, err := lineTotal.Sub(discount)
		if err != nil {
			return PricingSummary{}, nil, mapMoneyError(err)
		}
		tax, taxRule, err := applyTaxRule(taxBase, item, request.Region)
		if err != nil {
			return PricingSummary{}, nil, err
		}
		rules = append(rules, taxRule)
		taxTotal, err = taxTotal.Add(tax)
		if err != nil {
			return PricingSummary{}, nil, mapMoneyError(err)
		}
	}
	total, err := subtotal.Sub(discountTotal)
	if err != nil {
		return PricingSummary{}, nil, mapMoneyError(err)
	}
	total, err = total.Add(taxTotal)
	if err != nil {
		return PricingSummary{}, nil, mapMoneyError(err)
	}
	total, err = total.Round()
	if err != nil {
		return PricingSummary{}, nil, mapMoneyError(err)
	}
	return PricingSummary{
		Currency:      currency.Code(),
		Subtotal:      moneyValue(subtotal),
		DiscountTotal: moneyValue(discountTotal),
		TaxTotal:      moneyValue(taxTotal),
		Total:         moneyValue(total),
		Lines:         lines,
	}, rules, nil
}

func applyDiscountRule(lineTotal money.Money, item LineItemRequest, customerTier string) (money.Money, RuleDecision, error) {
	zero, err := roundedZero(lineTotal.Currency())
	if err != nil {
		return money.Money{}, RuleDecision{}, err
	}
	rule := RuleDecision{
		Name:     "vip-service-discount",
		LineID:   item.LineID,
		Currency: lineTotal.Currency().Code(),
		Status:   RuleSkipped,
	}
	if !strings.EqualFold(strings.TrimSpace(customerTier), "vip") {
		rule.Reason = "tier_not_eligible"
		return zero, rule, nil
	}
	if !strings.EqualFold(item.Category, "service") {
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

func applyTaxRule(taxBase money.Money, item LineItemRequest, region string) (money.Money, RuleDecision, error) {
	zero, err := roundedZero(taxBase.Currency())
	if err != nil {
		return money.Money{}, RuleDecision{}, err
	}
	rule := RuleDecision{
		Name:     "regional-vat",
		LineID:   item.LineID,
		Currency: taxBase.Currency().Code(),
		Status:   RuleSkipped,
	}
	if !strings.EqualFold(strings.TrimSpace(region), "EU") {
		rule.Reason = "region_not_eu"
		return zero, rule, nil
	}
	if strings.EqualFold(item.Category, "tax_exempt") {
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

// NewRouter creates the Gin router for the checkout guard API.
func NewRouter(service *Service) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	_ = router.SetTrustedProxies(nil)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/tokens", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodySize)
		var request TokenRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			writeError(c, ErrInvalidRequest)
			return
		}
		response, err := service.IssueToken(request)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, response)
	})
	router.POST("/checkout/guard", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodySize)
		var request CheckoutRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			writeError(c, ErrInvalidRequest)
			return
		}
		response, err := service.GuardCheckout(c.GetHeader("Authorization"), request)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, response)
	})
	return router
}

func defaultConfig() (Config, error) {
	generator, err := id.NewUUIDV7Generator()
	if err != nil {
		return Config{}, fmt.Errorf("%w: create id generator: %w", ErrGuard, err)
	}
	return Config{
		issuer:                   DefaultIssuer,
		accessAudience:           AccessAudience,
		requiredRole:             defaultRole,
		requiredScope:            RequiredScope,
		secret:                   []byte("checkout-guard-integration-demo-secret"),
		clock:                    time.Now,
		idGenerator:              generator,
		expectedInsertions:       defaultExpectedN,
		falsePositiveProbability: defaultTargetFPP,
		scenarioName:             defaultScenario,
	}, nil
}

func validateTokenRequest(request TokenRequest) error {
	if strings.TrimSpace(request.Subject) == "" {
		return fmt.Errorf("%w: subject is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(request.Role) == "" {
		return fmt.Errorf("%w: role is required", ErrInvalidRequest)
	}
	if len(normalizeScopes(request.Scopes)) == 0 {
		return fmt.Errorf("%w: at least one scope is required", ErrInvalidRequest)
	}
	if request.TTLSeconds <= 0 {
		return fmt.Errorf("%w: ttl_seconds must be positive", ErrInvalidRequest)
	}
	return nil
}

func normalizeCheckoutRequest(request CheckoutRequest) (CheckoutRequest, money.Currency, error) {
	checkoutID := strings.TrimSpace(request.CheckoutID)
	if checkoutID == "" {
		return CheckoutRequest{}, money.Currency{}, fmt.Errorf("%w: checkout_id is required", ErrInvalidRequest)
	}
	idempotencyKey := strings.TrimSpace(request.IdempotencyKey)
	if idempotencyKey == "" {
		return CheckoutRequest{}, money.Currency{}, fmt.Errorf("%w: idempotency_key is required", ErrInvalidRequest)
	}
	if len(request.Items) == 0 {
		return CheckoutRequest{}, money.Currency{}, fmt.Errorf("%w: at least one item is required", ErrInvalidRequest)
	}
	currency, err := money.ParseCurrency(strings.TrimSpace(request.Currency))
	if err != nil {
		return CheckoutRequest{}, money.Currency{}, mapMoneyError(err)
	}
	items := make([]LineItemRequest, 0, len(request.Items))
	for _, item := range request.Items {
		item.LineID = strings.TrimSpace(item.LineID)
		item.SKU = strings.TrimSpace(item.SKU)
		item.UnitPrice = strings.TrimSpace(item.UnitPrice)
		item.Currency = strings.TrimSpace(item.Currency)
		item.Category = strings.TrimSpace(item.Category)
		if item.LineID == "" {
			return CheckoutRequest{}, money.Currency{}, fmt.Errorf("%w: line_id is required", ErrInvalidRequest)
		}
		if item.SKU == "" {
			return CheckoutRequest{}, money.Currency{}, fmt.Errorf("%w: sku is required", ErrInvalidRequest)
		}
		if item.UnitPrice == "" {
			return CheckoutRequest{}, money.Currency{}, fmt.Errorf("%w: unit_price is required", ErrInvalidRequest)
		}
		if item.Quantity <= 0 {
			return CheckoutRequest{}, money.Currency{}, fmt.Errorf("%w: quantity must be positive", ErrInvalidRequest)
		}
		lineCurrency, err := money.ParseCurrency(item.Currency)
		if err != nil {
			return CheckoutRequest{}, money.Currency{}, mapMoneyError(err)
		}
		if lineCurrency.Code() != currency.Code() {
			return CheckoutRequest{}, money.Currency{}, fmt.Errorf("%w: checkout uses one settlement currency", ErrInvalidMoney)
		}
		items = append(items, item)
	}
	request.CheckoutID = checkoutID
	request.IdempotencyKey = idempotencyKey
	request.CustomerTier = strings.TrimSpace(request.CustomerTier)
	request.Region = strings.TrimSpace(request.Region)
	request.Currency = currency.Code()
	request.Items = items
	return request, currency, nil
}

func bearerToken(authorization string) (string, error) {
	if strings.TrimSpace(authorization) == "" {
		return "", ErrMissingToken
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authorization, prefix) {
		return "", fmt.Errorf("%w: Authorization must use Bearer scheme", ErrInvalidToken)
	}
	token := strings.TrimSpace(strings.TrimPrefix(authorization, prefix))
	if token == "" {
		return "", ErrMissingToken
	}
	return token, nil
}

func normalizeScopes(scopes []string) []string {
	normalized := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			normalized = append(normalized, scope)
		}
	}
	return normalized
}

func hasScope(scopeClaim, required string) bool {
	for _, scope := range strings.Fields(scopeClaim) {
		if scope == required {
			return true
		}
	}
	return false
}

func (s *Service) nextID() (string, error) {
	id, err := s.idGenerator.NextString()
	if err != nil {
		return "", fmt.Errorf("%w: generate id: %w", ErrGuard, err)
	}
	return id, nil
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

func writeError(c *gin.Context, err error) {
	status, response := publicError(err)
	c.JSON(status, response)
}

func publicError(err error) (int, ErrorResponse) {
	switch {
	case errors.Is(err, ErrMissingToken):
		return http.StatusUnauthorized, ErrorResponse{
			ErrorCode: "missing_token",
			Message:   "A bearer token is required.",
		}
	case errors.Is(err, ErrExpiredToken):
		return http.StatusUnauthorized, ErrorResponse{
			ErrorCode: "expired_token",
			Message:   "The token has expired.",
		}
	case errors.Is(err, ErrInvalidToken):
		return http.StatusUnauthorized, ErrorResponse{
			ErrorCode: "invalid_token",
			Message:   "The token could not be verified.",
		}
	case errors.Is(err, ErrInvalidClaims):
		return http.StatusForbidden, ErrorResponse{
			ErrorCode: "invalid_claims",
			Message:   "The verified token is not allowed for checkout.",
		}
	case errors.Is(err, ErrInvalidRequest):
		return http.StatusBadRequest, ErrorResponse{
			ErrorCode: "invalid_request",
			Message:   "The request body is invalid.",
		}
	case errors.Is(err, ErrInvalidMoney):
		return http.StatusBadRequest, ErrorResponse{
			ErrorCode: "invalid_money",
			Message:   "The money input is invalid.",
		}
	case errors.Is(err, ErrRuleDenied):
		return http.StatusUnprocessableEntity, ErrorResponse{
			ErrorCode: "rule_denied",
			Message:   "A checkout eligibility rule denied the submission.",
		}
	case errors.Is(err, ErrDuplicateSubmission):
		return http.StatusConflict, ErrorResponse{
			ErrorCode: "duplicate_submission",
			Message:   "The checkout was already seen or may be a false positive.",
		}
	default:
		return http.StatusInternalServerError, ErrorResponse{
			ErrorCode: "checkout_error",
			Message:   "The checkout guard could not process the request.",
		}
	}
}
