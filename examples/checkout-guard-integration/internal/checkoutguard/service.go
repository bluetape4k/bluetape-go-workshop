// Package checkoutguard 는 여러 유틸리티를 조합한 checkout guard 예제를 구현한다.
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
	// DefaultIssuer 는 checkout guard가 기대하는 데모 issuer다.
	DefaultIssuer = "checkout-guard-integration"
	// AccessAudience 는 checkout access token에 필요한 audience다.
	AccessAudience = "checkout-api"
	// TokenUseAccess 는 checkout guard가 허용하는 token 용도를 표시한다.
	TokenUseAccess = "access"
	// RequiredScope 는 보호되는 checkout 제출에 필요한 scope다.
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
	// ErrMissingToken 은 bearer token이 누락되었음을 나타낸다.
	ErrMissingToken = errors.New("checkoutguard: missing token")
	// ErrInvalidToken 은 형식이 잘못되었거나 검증할 수 없는 bearer token을 나타낸다.
	ErrInvalidToken = errors.New("checkoutguard: invalid token")
	// ErrExpiredToken 은 검증은 되었지만 만료 시간이 지난 token을 나타낸다.
	ErrExpiredToken = errors.New("checkoutguard: expired token")
	// ErrInvalidClaims 는 검증된 token의 claim 계약이 기대와 다름을 나타낸다.
	ErrInvalidClaims = errors.New("checkoutguard: invalid claims")
	// ErrInvalidRequest 는 요청 JSON 또는 필드가 유효하지 않음을 나타낸다.
	ErrInvalidRequest = errors.New("checkoutguard: invalid request")
	// ErrInvalidMoney 는 money 값이 유효하지 않거나 currency가 맞지 않음을 나타낸다.
	ErrInvalidMoney = errors.New("checkoutguard: invalid money")
	// ErrRuleDenied 는 로컬 checkout eligibility rule이 요청을 거부했음을 나타낸다.
	ErrRuleDenied = errors.New("checkoutguard: rule denied")
	// ErrDuplicateSubmission 은 반복되었거나 반복되었을 가능성이 있는 idempotency key를 나타낸다.
	ErrDuplicateSubmission = errors.New("checkoutguard: duplicate submission")
	// ErrGuard 는 예상하지 못한 guard 구성 또는 런타임 실패를 나타낸다.
	ErrGuard = errors.New("checkoutguard: guard error")
)

type stringGenerator interface {
	NextString() (string, error)
}

// Decision 은 공개 admission 판단값이다.
type Decision string

const (
	// DecisionAdmit 은 삽입 전 idempotency key가 확실히 없었다는 뜻이다.
	DecisionAdmit Decision = "admit"
	// DecisionProbablySeen 은 idempotency key가 이전에 관측되었을 수 있다는 뜻이다.
	DecisionProbablySeen Decision = "probably_seen"
)

const (
	// ReasonDefinitelyNew 는 false negative가 없는 Bloom filter 경로를 설명한다.
	ReasonDefinitelyNew = "definitely_new"
	// ReasonMightBeDuplicate 은 중복 가능성 또는 false positive 경로를 설명한다.
	ReasonMightBeDuplicate = "might_be_duplicate_or_false_positive"
)

// RuleStatus 는 로컬 checkout rule의 공개 상태값이다.
type RuleStatus string

const (
	// RuleAccepted 는 rule이 적용되어 checkout을 변경했거나 승인했다는 뜻이다.
	RuleAccepted RuleStatus = "accepted"
	// RuleSkipped 는 rule이 적용 대상이 아니었다는 뜻이다.
	RuleSkipped RuleStatus = "skipped"
	// RuleDenied 는 rule이 checkout을 차단했다는 뜻이다.
	RuleDenied RuleStatus = "denied"
)

// Config 는 데모 guard를 구성하는 서비스 연결값을 보관한다.
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

// Option 은 서비스 생성 전에 Config를 조정한다.
type Option func(*Config)

// WithClock 은 token 발급과 parsing 검증에 사용할 서비스 시계를 설정한다.
func WithClock(clock func() time.Time) Option {
	return func(cfg *Config) {
		if clock != nil {
			cfg.clock = clock
		}
	}
}

// WithSecret 은 고정 HMAC 데모 secret을 설정한다.
func WithSecret(secret []byte) Option {
	return func(cfg *Config) {
		cfg.secret = append([]byte(nil), secret...)
	}
}

// WithIDGenerator 는 session, JWT ID, request, order에 사용할 ID generator를 설정한다.
func WithIDGenerator(generator stringGenerator) Option {
	return func(cfg *Config) {
		if generator != nil {
			cfg.idGenerator = generator
		}
	}
}

// WithBloomConfig 는 예상 Bloom filter 크기와 목표 false-positive probability를 설정한다.
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

// Service 는 token 발급, token 검증, money rule, Bloom admission을 소유한다.
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

// TokenRequest 는 로컬 데모 access-token 발급 요청이다.
type TokenRequest struct {
	Subject    string   `json:"subject"`
	Role       string   `json:"role"`
	Scopes     []string `json:"scopes"`
	TTLSeconds int      `json:"ttl_seconds"`
}

// TokenResponse 는 로컬 데모 access-token 발급 응답이다.
type TokenResponse struct {
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	Token            string `json:"token"`
	SessionID        string `json:"session_id"`
}

// CheckoutRequest 는 보호되는 checkout 제출 요청이다.
type CheckoutRequest struct {
	CheckoutID     string            `json:"checkout_id"`
	IdempotencyKey string            `json:"idempotency_key"`
	CustomerTier   string            `json:"customer_tier"`
	Region         string            `json:"region"`
	Currency       string            `json:"currency"`
	Items          []LineItemRequest `json:"items"`
}

// LineItemRequest 는 checkout line 하나를 나타낸다.
type LineItemRequest struct {
	LineID    string `json:"line_id"`
	SKU       string `json:"sku"`
	UnitPrice string `json:"unit_price"`
	Currency  string `json:"currency"`
	Quantity  int    `json:"quantity"`
	Category  string `json:"category"`
}

// CheckoutResponse 는 호출자에게 반환되는 승인된 checkout 프로젝션이다.
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

// AdmissionDecision 은 안정적인 공개 Bloom 판단 프로젝션이다.
type AdmissionDecision struct {
	IdempotencyKey string      `json:"idempotency_key"`
	Scenario       string      `json:"scenario"`
	Decision       Decision    `json:"decision"`
	Reason         string      `json:"reason"`
	Accepted       bool        `json:"accepted"`
	Stats          FilterStats `json:"stats"`
}

// FilterStats 는 대략적인 Bloom filter 상태를 보고한다.
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

// PricingSummary 는 하나의 checkout currency에 대해 반올림된 money 합계를 보고한다.
type PricingSummary struct {
	Currency      string      `json:"currency"`
	Subtotal      MoneyValue  `json:"subtotal"`
	DiscountTotal MoneyValue  `json:"discount_total"`
	TaxTotal      MoneyValue  `json:"tax_total"`
	Total         MoneyValue  `json:"total"`
	Lines         []LineQuote `json:"lines"`
}

// LineQuote 는 반올림된 line 합계를 보고한다.
type LineQuote struct {
	LineID    string     `json:"line_id"`
	SKU       string     `json:"sku"`
	Category  string     `json:"category"`
	Quantity  int        `json:"quantity"`
	LineTotal MoneyValue `json:"line_total"`
}

// RuleDecision 은 안정적인 공개 local-rule 프로젝션이다.
type RuleDecision struct {
	Name     string      `json:"name"`
	LineID   string      `json:"line_id,omitempty"`
	Status   RuleStatus  `json:"status"`
	Currency string      `json:"currency,omitempty"`
	Amount   *MoneyValue `json:"amount,omitempty"`
	Reason   string      `json:"reason,omitempty"`
}

// MoneyValue 는 JSON에 안전하게 담을 수 있는 money 표현이다.
type MoneyValue struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// ErrorResponse 는 안정적인 공개 오류 형식이다.
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

// NewService 는 결정적인 로컬 JWT와 Bloom 기본값으로 데모 서비스를 만든다.
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

// IssueToken 은 checkout guard에 사용할 로컬 데모 access token을 발급한다.
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

// GuardCheckout 은 claim을 검증하고 checkout 가격을 계산한 뒤 idempotency key를 admission 처리한다.
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

// NewRouter 는 checkout guard API용 Gin router를 만든다.
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
