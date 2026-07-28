// Package idjwtboundary 는 ID/JWT trust-boundary 워크숍 예제를 구현한다.
package idjwtboundary

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/id"
	btjwt "github.com/bluetape4k/bluetape-go/jwt"
	"github.com/gin-gonic/gin"
)

const (
	// DefaultIssuer 는 order boundary가 기대하는 데모 issuer다.
	DefaultIssuer = "id-jwt-boundary"
	// DefaultAudience 는 order boundary가 기대하는 데모 audience다.
	DefaultAudience = "orders-api"

	defaultRole     = "customer"
	defaultScope    = "orders:create"
	defaultTokenTTL = 15 * time.Minute
	maxJSONBodySize = 8 << 10
)

var (
	// ErrMissingToken 은 bearer token이 누락되었음을 나타낸다.
	ErrMissingToken = errors.New("idjwtboundary: missing token")
	// ErrInvalidToken 은 형식이 잘못되었거나 검증할 수 없는 bearer token을 나타낸다.
	ErrInvalidToken = errors.New("idjwtboundary: invalid token")
	// ErrExpiredToken 은 검증은 되었지만 만료 시간이 지난 token을 나타낸다.
	ErrExpiredToken = errors.New("idjwtboundary: expired token")
	// ErrForbidden 은 필수 role 또는 scope가 없는 검증된 token을 나타낸다.
	ErrForbidden = errors.New("idjwtboundary: forbidden")
	// ErrInvalidRequest 는 요청 JSON 또는 field가 유효하지 않음을 나타낸다.
	ErrInvalidRequest = errors.New("idjwtboundary: invalid request")

	errInjectedIDFailure = errors.New("injected id failure")
)

type stringGenerator interface {
	NextString() (string, error)
}

// Config 는 데모 boundary를 구성하는 service wiring을 보관한다.
type Config struct {
	issuer        string
	audience      string
	requiredRole  string
	requiredScope string
	secret        []byte
	clock         func() time.Time
	idGenerator   stringGenerator
}

// Option 은 service 생성 전에 Config를 조정한다.
type Option func(*Config)

// WithClock 은 token 발급과 parse 검증에 사용할 service clock을 설정한다.
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

// WithIDGenerator 는 내부 order/request ID에 사용할 ID generator를 설정한다.
func WithIDGenerator(generator stringGenerator) Option {
	return func(cfg *Config) {
		if generator != nil {
			cfg.idGenerator = generator
		}
	}
}

// Service 는 token 발급, token 검증, 내부 ID 생성을 소유한다.
type Service struct {
	provider      *btjwt.Provider
	idGenerator   stringGenerator
	clock         func() time.Time
	issuer        string
	audience      string
	requiredRole  string
	requiredScope string
}

// TokenRequest 는 로컬 데모 token 발급 요청이다.
type TokenRequest struct {
	Subject    string   `json:"subject"`
	Role       string   `json:"role"`
	Scopes     []string `json:"scopes"`
	TTLSeconds int      `json:"ttl_seconds"`
}

// TokenResponse 는 로컬 데모 token 발급 응답이다.
type TokenResponse struct {
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	Token            string `json:"token"`
}

// OrderRequest 는 보호되는 order intake 요청이다.
type OrderRequest struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

// OrderResponse 는 호출자에게 반환되는 accepted order projection이다.
type OrderResponse struct {
	OrderID   string `json:"order_id"`
	RequestID string `json:"request_id"`
	Subject   string `json:"subject"`
	Role      string `json:"role"`
	Scope     string `json:"scope"`
	SKU       string `json:"sku"`
	Quantity  int    `json:"quantity"`
}

// ErrorResponse 는 안정적인 공개 오류 형식이다.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

type tokenClaims struct {
	subject string
	role    string
	scope   string
}

// NewService 는 결정적인 local JWT 기본값으로 데모 service를 만든다.
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
		return nil, err
	}
	return &Service{
		provider:      provider,
		idGenerator:   cfg.idGenerator,
		clock:         cfg.clock,
		issuer:        cfg.issuer,
		audience:      cfg.audience,
		requiredRole:  cfg.requiredRole,
		requiredScope: cfg.requiredScope,
	}, nil
}

func defaultConfig() (Config, error) {
	generator, err := id.NewUUIDV7Generator()
	if err != nil {
		return Config{}, err
	}
	return Config{
		issuer:        DefaultIssuer,
		audience:      DefaultAudience,
		requiredRole:  defaultRole,
		requiredScope: defaultScope,
		secret:        []byte("local-demo-secret-32-byte-value!!"),
		clock:         func() time.Time { return time.Now().UTC() },
		idGenerator:   generator,
	}, nil
}

// IssueToken 은 짧게 살아 있는 local demo token에 서명한다.
func (s *Service) IssueToken(request TokenRequest) (TokenResponse, error) {
	if err := validateTokenRequest(request); err != nil {
		return TokenResponse{}, err
	}
	ttl := time.Duration(request.TTLSeconds) * time.Second
	token, err := s.provider.Compose(
		btjwt.WithIssuer(s.issuer),
		btjwt.WithSubject(strings.TrimSpace(request.Subject)),
		btjwt.WithAudience(s.audience),
		btjwt.WithExpiresAfter(ttl),
		btjwt.WithClaim("role", strings.TrimSpace(request.Role)),
		btjwt.WithClaim("scope", strings.Join(normalizeScopes(request.Scopes), " ")),
	)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("%w: compose token", ErrInvalidToken)
	}
	return TokenResponse{
		TokenType:        "Bearer",
		ExpiresInSeconds: request.TTLSeconds,
		Token:            token,
	}, nil
}

// CreateOrder 는 bearer token을 검증하고 내부 order ID를 생성한다.
func (s *Service) CreateOrder(authorization string, request OrderRequest) (OrderResponse, error) {
	claims, err := s.verifyAuthorization(authorization)
	if err != nil {
		return OrderResponse{}, err
	}
	if err := validateOrderRequest(request); err != nil {
		return OrderResponse{}, err
	}

	orderID, err := s.idGenerator.NextString()
	if err != nil {
		return OrderResponse{}, fmt.Errorf("generate order id: %w", err)
	}
	requestID, err := s.idGenerator.NextString()
	if err != nil {
		return OrderResponse{}, fmt.Errorf("generate request id: %w", err)
	}

	return OrderResponse{
		OrderID:   orderID,
		RequestID: requestID,
		Subject:   claims.subject,
		Role:      claims.role,
		Scope:     s.requiredScope,
		SKU:       strings.TrimSpace(request.SKU),
		Quantity:  request.Quantity,
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
		btjwt.WithExpectedAudience(s.audience),
		btjwt.WithExpirationRequired(),
		btjwt.WithParseClock(s.clock),
	)
	if err != nil {
		if errors.Is(err, btjwt.ErrExpiredToken) {
			return tokenClaims{}, fmt.Errorf("%w: token expired", ErrExpiredToken)
		}
		return tokenClaims{}, fmt.Errorf("%w: token could not be verified", ErrInvalidToken)
	}

	role, _ := reader.ClaimString("role")
	scope, _ := reader.ClaimString("scope")
	claims := tokenClaims{
		subject: reader.Subject(),
		role:    role,
		scope:   scope,
	}
	if claims.subject == "" || claims.role != s.requiredRole || !hasScope(claims.scope, s.requiredScope) {
		return tokenClaims{}, fmt.Errorf("%w: required role or scope missing", ErrForbidden)
	}
	return claims, nil
}

// NewRouter 는 ID/JWT boundary API용 Gin router를 만든다.
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
	router.POST("/orders", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodySize)
		var request OrderRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			writeError(c, ErrInvalidRequest)
			return
		}
		response, err := service.CreateOrder(c.GetHeader("Authorization"), request)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, response)
	})
	return router
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

func validateOrderRequest(request OrderRequest) error {
	if strings.TrimSpace(request.SKU) == "" {
		return fmt.Errorf("%w: sku is required", ErrInvalidRequest)
	}
	if request.Quantity <= 0 {
		return fmt.Errorf("%w: quantity must be positive", ErrInvalidRequest)
	}
	return nil
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
			Message:   "The bearer token has expired.",
		}
	case errors.Is(err, ErrInvalidToken):
		return http.StatusUnauthorized, ErrorResponse{
			ErrorCode: "invalid_token",
			Message:   "The bearer token could not be verified.",
		}
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden, ErrorResponse{
			ErrorCode: "forbidden",
			Message:   "The verified token is not allowed to create orders.",
		}
	case errors.Is(err, ErrInvalidRequest):
		return http.StatusBadRequest, ErrorResponse{
			ErrorCode: "invalid_request",
			Message:   "The request body is invalid.",
		}
	default:
		return http.StatusInternalServerError, ErrorResponse{
			ErrorCode: "internal_error",
			Message:   "The order boundary could not process the request.",
		}
	}
}
