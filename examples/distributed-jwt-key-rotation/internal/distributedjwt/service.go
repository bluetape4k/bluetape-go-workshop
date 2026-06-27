package distributedjwt

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	btjwt "github.com/bluetape4k/bluetape-go/jwt"
	redisjwt "github.com/bluetape4k/bluetape-go/jwt/redis"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	// DefaultIssuer is the issuer expected by distributed JWT demo tokens.
	DefaultIssuer = "distributed-jwt-key-rotation"
	// DefaultAudience is the protected API audience expected by the demo.
	DefaultAudience = "orders-api"

	defaultNamespace        = "workshop-auth"
	defaultNodeID           = "api"
	defaultOperationTimeout = 750 * time.Millisecond
	defaultKeyTTL           = 24 * time.Hour
	defaultTokenTTL         = 15 * time.Minute
	maxTokenTTL             = time.Hour
	maxJSONBodySize         = 8 << 10
)

var (
	// ErrInvalidRequest reports invalid public request fields or JSON.
	ErrInvalidRequest = errors.New("distributedjwt: invalid request")
	// ErrInvalidToken reports a token that cannot be verified by this boundary.
	ErrInvalidToken = errors.New("distributedjwt: invalid token")
	// ErrExpiredToken reports a verified token that is past expiration.
	ErrExpiredToken = errors.New("distributedjwt: expired token")
	// ErrMissingToken reports a missing bearer token.
	ErrMissingToken = errors.New("distributedjwt: missing token")
)

// Config holds service wiring for the distributed JWT example.
type Config struct {
	issuer           string
	audience         string
	namespace        string
	nodeID           string
	clock            func() time.Time
	keyTTL           time.Duration
	operationTimeout time.Duration
}

// Option customizes Config before the service is constructed.
type Option func(*Config)

// WithIssuer sets the expected JWT issuer.
func WithIssuer(issuer string) Option {
	return func(cfg *Config) {
		if text := strings.TrimSpace(issuer); text != "" {
			cfg.issuer = text
		}
	}
}

// WithAudience sets the expected JWT audience.
func WithAudience(audience string) Option {
	return func(cfg *Config) {
		if text := strings.TrimSpace(audience); text != "" {
			cfg.audience = text
		}
	}
}

// WithNamespace sets the Redis key namespace for shared signing keys.
func WithNamespace(namespace string) Option {
	return func(cfg *Config) {
		if text := strings.TrimSpace(namespace); text != "" {
			cfg.namespace = text
		}
	}
}

// WithNodeID sets the demo node identifier used in generated key and token IDs.
func WithNodeID(nodeID string) Option {
	return func(cfg *Config) {
		if text := strings.TrimSpace(nodeID); text != "" {
			cfg.nodeID = text
		}
	}
}

// WithClock sets the service clock used for token issue, parse, and rotation.
func WithClock(clock func() time.Time) Option {
	return func(cfg *Config) {
		if clock != nil {
			cfg.clock = clock
		}
	}
}

// WithKeyTTL sets the retention window for repository-backed signing keys.
func WithKeyTTL(ttl time.Duration) Option {
	return func(cfg *Config) {
		if ttl > 0 {
			cfg.keyTTL = ttl
		}
	}
}

// WithOperationTimeout sets the child context budget for repository operations.
func WithOperationTimeout(timeout time.Duration) Option {
	return func(cfg *Config) {
		if timeout > 0 {
			cfg.operationTimeout = timeout
		}
	}
}

// Service owns distributed token issue, verification, and key rotation.
type Service struct {
	provider         *btjwt.CachedDistributedProvider
	issuer           string
	audience         string
	nodeID           string
	clock            func() time.Time
	operationTimeout time.Duration
	nextKID          atomic.Uint64
	nextJTI          atomic.Uint64
}

// IssueRequest is the local demo access-token issue request.
type IssueRequest struct {
	Subject    string   `json:"subject"`
	Scopes     []string `json:"scopes"`
	TTLSeconds int      `json:"ttl_seconds"`
}

// TokenResponse is the issued bearer token and signing key projection.
type TokenResponse struct {
	TokenType        string `json:"token_type"`
	AccessToken      string `json:"access_token"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	KID              string `json:"kid"`
}

// RotationResponse describes the new current signing key after rotation.
type RotationResponse struct {
	KID       string    `json:"kid"`
	Previous  string    `json:"previous_kid,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ProfileResponse is the protected resource projection from verified claims.
type ProfileResponse struct {
	Subject          string `json:"subject"`
	Scope            string `json:"scope"`
	KID              string `json:"kid"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

// ErrorResponse is the stable public error shape.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// NewService builds the distributed JWT example service.
func NewService(ctx context.Context, client redis.Cmdable, options ...Option) (*Service, error) {
	cfg := defaultConfig()
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	if client == nil {
		return nil, fmt.Errorf("%w: redis client is required", ErrInvalidRequest)
	}
	repo, err := redisjwt.New(redisjwt.Options{
		Client:    client,
		Namespace: cfg.namespace,
		Capacity:  3,
		KeyTTL:    cfg.keyTTL,
	})
	if err != nil {
		return nil, err
	}
	service := &Service{
		issuer:           cfg.issuer,
		audience:         cfg.audience,
		nodeID:           cfg.nodeID,
		clock:            cfg.clock,
		operationTimeout: cfg.operationTimeout,
	}
	providerCtx, cancel := service.operationContext(ctx)
	defer cancel()
	provider, err := btjwt.NewDistributedHMACProvider(providerCtx, repo, btjwt.HS256,
		btjwt.WithClock(cfg.clock),
		btjwt.WithKeyTTL(cfg.keyTTL),
		btjwt.WithKeyIDGenerator(service.nextKeyID),
	)
	if err != nil {
		return nil, err
	}
	readerCache := cache.NewMemory[string, *btjwt.Reader]()
	cached, err := btjwt.NewCachedDistributedProvider(provider, readerCache,
		btjwt.WithCacheClock(cfg.clock),
		btjwt.WithCacheTrustScope(cfg.namespace),
	)
	if err != nil {
		return nil, err
	}
	service.provider = cached
	return service, nil
}

func defaultConfig() Config {
	return Config{
		issuer:           DefaultIssuer,
		audience:         DefaultAudience,
		namespace:        defaultNamespace,
		nodeID:           defaultNodeID,
		clock:            func() time.Time { return time.Now().UTC() },
		keyTTL:           defaultKeyTTL,
		operationTimeout: defaultOperationTimeout,
	}
}

// IssueToken signs an access token with the current distributed signing key.
func (s *Service) IssueToken(ctx context.Context, request IssueRequest) (TokenResponse, error) {
	if err := validateIssueRequest(request); err != nil {
		return TokenResponse{}, err
	}
	opCtx, cancel := s.operationContext(ctx)
	defer cancel()

	ttl := time.Duration(request.TTLSeconds) * time.Second
	token, err := s.provider.ComposeContext(opCtx,
		btjwt.WithIssuer(s.issuer),
		btjwt.WithAudience(s.audience),
		btjwt.WithSubject(strings.TrimSpace(request.Subject)),
		btjwt.WithExpiresAfter(ttl),
		btjwt.WithJWTID(s.nextJWTID()),
		btjwt.WithClaim("scope", strings.Join(normalizeScopes(request.Scopes), " ")),
	)
	if err != nil {
		return TokenResponse{}, mapError(err)
	}
	reader, err := s.provider.ParseContext(opCtx, token, s.parseOptions()...)
	if err != nil {
		return TokenResponse{}, mapError(err)
	}
	return TokenResponse{
		TokenType:        "Bearer",
		AccessToken:      token,
		ExpiresInSeconds: int(ttl / time.Second),
		KID:              reader.Kid(),
	}, nil
}

// VerifyToken verifies an access token and returns the protected profile view.
func (s *Service) VerifyToken(ctx context.Context, token string) (ProfileResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return ProfileResponse{}, ErrMissingToken
	}
	opCtx, cancel := s.operationContext(ctx)
	defer cancel()

	reader, err := s.provider.ParseContext(opCtx, token, s.parseOptions()...)
	if err != nil {
		return ProfileResponse{}, mapError(err)
	}
	scope, _ := reader.ClaimString("scope")
	return ProfileResponse{
		Subject:          reader.Subject(),
		Scope:            scope,
		KID:              reader.Kid(),
		ExpiresInSeconds: int(reader.RemainingTTL(s.clock()) / time.Second),
	}, nil
}

// RotateKey forces a new distributed signing key and clears local reader cache.
func (s *Service) RotateKey(ctx context.Context) (RotationResponse, error) {
	opCtx, cancel := s.operationContext(ctx)
	defer cancel()

	current, _ := s.provider.CurrentKeyChainContext(opCtx)
	rotated, err := s.provider.ForcedRotateContext(opCtx)
	if err != nil {
		return RotationResponse{}, mapError(err)
	}
	response := RotationResponse{KID: rotated.KID(), ExpiresAt: rotated.ExpiresAt()}
	if current != nil && current.KID() != rotated.KID() {
		response.Previous = current.KID()
	}
	return response, nil
}

func (s *Service) operationContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, s.operationTimeout)
}

func (s *Service) parseOptions() []btjwt.ParseOption {
	return []btjwt.ParseOption{
		btjwt.WithExpectedIssuer(s.issuer),
		btjwt.WithExpectedAudience(s.audience),
		btjwt.WithExpirationRequired(),
		btjwt.WithParseClock(s.clock),
	}
}

func (s *Service) nextKeyID() (string, error) {
	next := s.nextKID.Add(1)
	return fmt.Sprintf("%s-%06d", s.nodeID, next), nil
}

func (s *Service) nextJWTID() string {
	next := s.nextJTI.Add(1)
	return fmt.Sprintf("%s-jti-%06d", s.nodeID, next)
}

func validateIssueRequest(request IssueRequest) error {
	if strings.TrimSpace(request.Subject) == "" {
		return fmt.Errorf("%w: subject is required", ErrInvalidRequest)
	}
	if len(normalizeScopes(request.Scopes)) == 0 {
		return fmt.Errorf("%w: at least one scope is required", ErrInvalidRequest)
	}
	if request.TTLSeconds <= 0 {
		return fmt.Errorf("%w: ttl_seconds must be positive", ErrInvalidRequest)
	}
	ttl := time.Duration(request.TTLSeconds) * time.Second
	if ttl > maxTokenTTL {
		return fmt.Errorf("%w: ttl_seconds must be at most %d", ErrInvalidRequest, int(maxTokenTTL/time.Second))
	}
	return nil
}

func normalizeScopes(scopes []string) []string {
	seen := make(map[string]struct{}, len(scopes))
	normalized := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		normalized = append(normalized, scope)
	}
	return normalized
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, btjwt.ErrExpiredToken):
		return fmt.Errorf("%w: %w", ErrExpiredToken, err)
	case errors.Is(err, btjwt.ErrInvalidToken), errors.Is(err, btjwt.ErrKeyNotFound), errors.Is(err, btjwt.ErrInvalidKey):
		return fmt.Errorf("%w: %w", ErrInvalidToken, err)
	default:
		return err
	}
}

// NewRouter builds the HTTP surface for token issue, verification, and rotation.
func NewRouter(service *Service) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/tokens", func(c *gin.Context) {
		var request IssueRequest
		if err := bindJSON(c, &request); err != nil {
			writeError(c, err)
			return
		}
		response, err := service.IssueToken(c.Request.Context(), request)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, response)
	})
	router.POST("/keys/rotate", func(c *gin.Context) {
		response, err := service.RotateKey(c.Request.Context())
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, response)
	})
	router.GET("/profile", func(c *gin.Context) {
		token, err := bearerToken(c.GetHeader("Authorization"))
		if err != nil {
			writeError(c, err)
			return
		}
		response, err := service.VerifyToken(c.Request.Context(), token)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, response)
	})
	return router
}

func bindJSON(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodySize)
	if err := c.ShouldBindJSON(target); err != nil {
		return fmt.Errorf("%w: malformed json", ErrInvalidRequest)
	}
	return nil
}

func bearerToken(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", ErrMissingToken
	}
	prefix := "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", ErrMissingToken
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", ErrMissingToken
	}
	return token, nil
}

func writeError(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, ErrInvalidRequest):
		status, code = http.StatusBadRequest, "invalid_request"
	case errors.Is(err, ErrMissingToken), errors.Is(err, ErrInvalidToken), errors.Is(err, ErrExpiredToken):
		status, code = http.StatusUnauthorized, errorCode(err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status, code = http.StatusGatewayTimeout, "repository_timeout"
	}
	c.JSON(status, ErrorResponse{ErrorCode: code, Message: code})
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, ErrMissingToken):
		return "missing_token"
	case errors.Is(err, ErrExpiredToken):
		return "expired_token"
	case errors.Is(err, ErrInvalidToken):
		return "invalid_token"
	default:
		return "internal_error"
	}
}
