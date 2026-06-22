// Package tokenrefresh implements the JWT claims and refresh-token workshop example.
package tokenrefresh

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	btjwt "github.com/bluetape4k/bluetape-go/jwt"
	"github.com/gin-gonic/gin"
)

const (
	// DefaultIssuer is the demo issuer expected by the token boundary.
	DefaultIssuer = "token-refresh-claims"
	// AccessAudience is the audience required for access tokens.
	AccessAudience = "session-api"
	// RefreshAudience is the audience required for refresh tokens.
	RefreshAudience = "token-refresh"
	// TokenUseAccess marks tokens accepted by protected resources.
	TokenUseAccess = "access"
	// TokenUseRefresh marks tokens accepted only by the refresh endpoint.
	TokenUseRefresh = "refresh"

	defaultRole       = "customer"
	defaultScope      = "profile:read"
	defaultAccessTTL  = 5 * time.Minute
	defaultRefreshTTL = 24 * time.Hour
	maxJSONBodySize   = 8 << 10
)

var (
	// ErrMissingToken reports a missing bearer token.
	ErrMissingToken = errors.New("tokenrefresh: missing token")
	// ErrInvalidToken reports a malformed or unverifiable token.
	ErrInvalidToken = errors.New("tokenrefresh: invalid token")
	// ErrExpiredToken reports a verified token that is past expiration.
	ErrExpiredToken = errors.New("tokenrefresh: expired token")
	// ErrInvalidClaims reports a verified token with the wrong claim contract.
	ErrInvalidClaims = errors.New("tokenrefresh: invalid claims")
	// ErrInvalidRequest reports invalid request JSON or fields.
	ErrInvalidRequest = errors.New("tokenrefresh: invalid request")

	errInjectedIDFailure = errors.New("injected id failure")
)

type stringGenerator interface {
	NextString() (string, error)
}

// Config holds service wiring for the demo boundary.
type Config struct {
	issuer          string
	accessAudience  string
	refreshAudience string
	requiredRole    string
	requiredScope   string
	secret          []byte
	clock           func() time.Time
	idGenerator     stringGenerator
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

// WithIDGenerator sets the ID generator used for session IDs and JWT IDs.
func WithIDGenerator(generator stringGenerator) Option {
	return func(cfg *Config) {
		if generator != nil {
			cfg.idGenerator = generator
		}
	}
}

// Service owns token issue, token verification, and refresh-token exchange.
type Service struct {
	provider        *btjwt.Provider
	idGenerator     stringGenerator
	clock           func() time.Time
	issuer          string
	accessAudience  string
	refreshAudience string
	requiredRole    string
	requiredScope   string
}

// SessionRequest is the local demo session issue request.
type SessionRequest struct {
	Subject    string   `json:"subject"`
	Role       string   `json:"role"`
	Scopes     []string `json:"scopes"`
	TTLSeconds int      `json:"ttl_seconds"`
}

// SessionResponse is the issued access/refresh token pair.
type SessionResponse struct {
	TokenType               string `json:"token_type"`
	AccessExpiresInSeconds  int    `json:"access_expires_in_seconds"`
	RefreshExpiresInSeconds int    `json:"refresh_expires_in_seconds"`
	AccessToken             string `json:"access_token"`
	RefreshToken            string `json:"refresh_token"`
	SessionID               string `json:"session_id"`
}

// RefreshRequest exchanges a refresh token for a new access token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// AccessTokenResponse is the refresh endpoint response.
type AccessTokenResponse struct {
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	AccessToken      string `json:"access_token"`
	SessionID        string `json:"session_id"`
}

// ProfileResponse is the protected resource projection from verified claims.
type ProfileResponse struct {
	Subject                string `json:"subject"`
	Role                   string `json:"role"`
	Scope                  string `json:"scope"`
	SessionID              string `json:"session_id"`
	AccessExpiresInSeconds int    `json:"access_expires_in_seconds"`
}

// ErrorResponse is the stable public error shape.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

type accessClaims struct {
	subject   string
	role      string
	scope     string
	sessionID string
	expiresIn int
}

type refreshClaims struct {
	subject   string
	sessionID string
}

// NewService builds the demo service with deterministic local JWT defaults.
func NewService(options ...Option) (*Service, error) {
	cfg := defaultConfig()
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
		provider:        provider,
		idGenerator:     cfg.idGenerator,
		clock:           cfg.clock,
		issuer:          cfg.issuer,
		accessAudience:  cfg.accessAudience,
		refreshAudience: cfg.refreshAudience,
		requiredRole:    cfg.requiredRole,
		requiredScope:   cfg.requiredScope,
	}, nil
}

func defaultConfig() Config {
	return Config{
		issuer:          DefaultIssuer,
		accessAudience:  AccessAudience,
		refreshAudience: RefreshAudience,
		requiredRole:    defaultRole,
		requiredScope:   defaultScope,
		secret:          []byte("local-demo-secret-32-byte-value!!"),
		clock:           func() time.Time { return time.Now().UTC() },
		idGenerator:     randomIDGenerator{},
	}
}

// IssueSession signs a local demo access/refresh token pair.
func (s *Service) IssueSession(request SessionRequest) (SessionResponse, error) {
	if err := validateSessionRequest(request); err != nil {
		return SessionResponse{}, err
	}
	ttl := time.Duration(request.TTLSeconds) * time.Second
	subject := strings.TrimSpace(request.Subject)
	role := strings.TrimSpace(request.Role)
	scope := strings.Join(normalizeScopes(request.Scopes), " ")

	sessionID, err := s.nextID()
	if err != nil {
		return SessionResponse{}, err
	}
	accessToken, err := s.issueAccessToken(subject, role, scope, sessionID, ttl)
	if err != nil {
		return SessionResponse{}, err
	}
	refreshJTI, err := s.nextID()
	if err != nil {
		return SessionResponse{}, err
	}
	refreshToken, err := s.provider.Compose(
		btjwt.WithIssuer(s.issuer),
		btjwt.WithSubject(subject),
		btjwt.WithAudience(s.refreshAudience),
		btjwt.WithExpiresAfter(defaultRefreshTTL),
		btjwt.WithJWTID(refreshJTI),
		btjwt.WithClaim("token_use", TokenUseRefresh),
		btjwt.WithClaim("session_id", sessionID),
	)
	if err != nil {
		return SessionResponse{}, fmt.Errorf("%w: compose refresh token", ErrInvalidToken)
	}

	return SessionResponse{
		TokenType:               "Bearer",
		AccessExpiresInSeconds:  request.TTLSeconds,
		RefreshExpiresInSeconds: int(defaultRefreshTTL / time.Second),
		AccessToken:             accessToken,
		RefreshToken:            refreshToken,
		SessionID:               sessionID,
	}, nil
}

// ValidateAccess verifies the bearer access token and returns stable claim context.
func (s *Service) ValidateAccess(authorization string) (ProfileResponse, error) {
	token, err := bearerToken(authorization)
	if err != nil {
		return ProfileResponse{}, err
	}
	claims, err := s.parseAccessToken(token)
	if err != nil {
		return ProfileResponse{}, err
	}
	return ProfileResponse{
		Subject:                claims.subject,
		Role:                   claims.role,
		Scope:                  s.requiredScope,
		SessionID:              claims.sessionID,
		AccessExpiresInSeconds: claims.expiresIn,
	}, nil
}

// RefreshAccess verifies a refresh token and returns a new access token.
func (s *Service) RefreshAccess(request RefreshRequest) (AccessTokenResponse, error) {
	token := strings.TrimSpace(request.RefreshToken)
	if token == "" {
		return AccessTokenResponse{}, fmt.Errorf("%w: refresh_token is required", ErrInvalidRequest)
	}
	claims, err := s.parseRefreshToken(token)
	if err != nil {
		return AccessTokenResponse{}, err
	}
	accessToken, err := s.issueAccessToken(claims.subject, s.requiredRole, s.requiredScope, claims.sessionID, defaultAccessTTL)
	if err != nil {
		return AccessTokenResponse{}, err
	}
	return AccessTokenResponse{
		TokenType:        "Bearer",
		ExpiresInSeconds: int(defaultAccessTTL / time.Second),
		AccessToken:      accessToken,
		SessionID:        claims.sessionID,
	}, nil
}

func (s *Service) issueAccessToken(subject, role, scope, sessionID string, ttl time.Duration) (string, error) {
	jti, err := s.nextID()
	if err != nil {
		return "", err
	}
	token, err := s.provider.Compose(
		btjwt.WithIssuer(s.issuer),
		btjwt.WithSubject(subject),
		btjwt.WithAudience(s.accessAudience),
		btjwt.WithExpiresAfter(ttl),
		btjwt.WithJWTID(jti),
		btjwt.WithClaim("token_use", TokenUseAccess),
		btjwt.WithClaim("role", role),
		btjwt.WithClaim("scope", scope),
		btjwt.WithClaim("session_id", sessionID),
	)
	if err != nil {
		return "", fmt.Errorf("%w: compose access token", ErrInvalidToken)
	}
	return token, nil
}

func (s *Service) parseAccessToken(token string) (accessClaims, error) {
	reader, err := s.parseToken(token)
	if err != nil {
		return accessClaims{}, err
	}
	tokenUse, _ := reader.ClaimString("token_use")
	role, _ := reader.ClaimString("role")
	scope, _ := reader.ClaimString("scope")
	sessionID, _ := reader.ClaimString("session_id")
	claims := accessClaims{
		subject:   reader.Subject(),
		role:      role,
		scope:     scope,
		sessionID: sessionID,
		expiresIn: int(reader.RemainingTTL(s.clock()).Seconds()),
	}
	if claims.subject == "" ||
		claims.sessionID == "" ||
		tokenUse != TokenUseAccess ||
		!hasAudience(reader.Audience(), s.accessAudience) ||
		claims.role != s.requiredRole ||
		!hasScope(claims.scope, s.requiredScope) {
		return accessClaims{}, fmt.Errorf("%w: access token claim contract mismatch", ErrInvalidClaims)
	}
	if claims.expiresIn < 0 {
		claims.expiresIn = 0
	}
	return claims, nil
}

func (s *Service) parseRefreshToken(token string) (refreshClaims, error) {
	reader, err := s.parseToken(token)
	if err != nil {
		return refreshClaims{}, err
	}
	tokenUse, _ := reader.ClaimString("token_use")
	sessionID, _ := reader.ClaimString("session_id")
	claims := refreshClaims{
		subject:   reader.Subject(),
		sessionID: sessionID,
	}
	if claims.subject == "" ||
		claims.sessionID == "" ||
		tokenUse != TokenUseRefresh ||
		!hasAudience(reader.Audience(), s.refreshAudience) {
		return refreshClaims{}, fmt.Errorf("%w: refresh token claim contract mismatch", ErrInvalidClaims)
	}
	return claims, nil
}

func (s *Service) parseToken(token string) (*btjwt.Reader, error) {
	reader, err := s.provider.Parse(
		token,
		btjwt.WithExpectedIssuer(s.issuer),
		btjwt.WithExpirationRequired(),
		btjwt.WithParseClock(s.clock),
	)
	if err != nil {
		if errors.Is(err, btjwt.ErrExpiredToken) {
			return nil, fmt.Errorf("%w: token expired", ErrExpiredToken)
		}
		return nil, fmt.Errorf("%w: token could not be verified", ErrInvalidToken)
	}
	return reader, nil
}

func (s *Service) nextID() (string, error) {
	id, err := s.idGenerator.NextString()
	if err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}
	return id, nil
}

// NewRouter creates the Gin router for the token refresh claims API.
func NewRouter(service *Service) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	_ = router.SetTrustedProxies(nil)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/sessions", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodySize)
		var request SessionRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			writeError(c, ErrInvalidRequest)
			return
		}
		response, err := service.IssueSession(request)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, response)
	})
	router.GET("/profile", func(c *gin.Context) {
		response, err := service.ValidateAccess(c.GetHeader("Authorization"))
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, response)
	})
	router.POST("/tokens/refresh", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodySize)
		var request RefreshRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			writeError(c, ErrInvalidRequest)
			return
		}
		response, err := service.RefreshAccess(request)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, response)
	})
	return router
}

func validateSessionRequest(request SessionRequest) error {
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

func hasAudience(audiences []string, required string) bool {
	for _, audience := range audiences {
		if audience == required {
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
			Message:   "The verified token is not allowed for this operation.",
		}
	case errors.Is(err, ErrInvalidRequest):
		return http.StatusBadRequest, ErrorResponse{
			ErrorCode: "invalid_request",
			Message:   "The request body is invalid.",
		}
	default:
		return http.StatusInternalServerError, ErrorResponse{
			ErrorCode: "internal_error",
			Message:   "The token boundary could not process the request.",
		}
	}
}

type randomIDGenerator struct{}

func (randomIDGenerator) NextString() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}
