// Package tokenrefresh 는 JWT claims 와 refresh-token 워크숍 예제를 구현한다.
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
	// DefaultIssuer 는 토큰 경계가 기대하는 데모 issuer 다.
	DefaultIssuer = "token-refresh-claims"
	// AccessAudience 는 access token 에 필요한 audience 다.
	AccessAudience = "session-api"
	// RefreshAudience 는 refresh token 에 필요한 audience 다.
	RefreshAudience = "token-refresh"
	// TokenUseAccess 는 보호 리소스가 수락하는 token 을 표시한다.
	TokenUseAccess = "access"
	// TokenUseRefresh 는 refresh endpoint 에서만 수락하는 token 을 표시한다.
	TokenUseRefresh = "refresh"

	defaultRole       = "customer"
	defaultScope      = "profile:read"
	defaultAccessTTL  = 5 * time.Minute
	defaultRefreshTTL = 24 * time.Hour
	maxJSONBodySize   = 8 << 10
)

var (
	// ErrMissingToken 은 bearer token 누락을 나타낸다.
	ErrMissingToken = errors.New("tokenrefresh: missing token")
	// ErrInvalidToken 은 형식이 잘못됐거나 검증할 수 없는 token 을 나타낸다.
	ErrInvalidToken = errors.New("tokenrefresh: invalid token")
	// ErrExpiredToken 은 검증은 됐지만 만료 시간이 지난 token 을 나타낸다.
	ErrExpiredToken = errors.New("tokenrefresh: expired token")
	// ErrInvalidClaims 는 검증됐지만 claim 계약이 맞지 않는 token 을 나타낸다.
	ErrInvalidClaims = errors.New("tokenrefresh: invalid claims")
	// ErrInvalidRequest 는 요청 JSON 또는 필드가 유효하지 않음을 나타낸다.
	ErrInvalidRequest = errors.New("tokenrefresh: invalid request")

	errInjectedIDFailure = errors.New("injected id failure")
)

type stringGenerator interface {
	NextString() (string, error)
}

// Config 는 데모 경계의 서비스 wiring 을 보관한다.
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

// Option 은 서비스 구성 전에 Config 를 조정한다.
type Option func(*Config)

// WithClock 은 token 발급과 parse 검사에 사용할 서비스 clock 을 설정한다.
func WithClock(clock func() time.Time) Option {
	return func(cfg *Config) {
		if clock != nil {
			cfg.clock = clock
		}
	}
}

// WithSecret 은 고정 HMAC 데모 secret 을 설정한다.
func WithSecret(secret []byte) Option {
	return func(cfg *Config) {
		cfg.secret = append([]byte(nil), secret...)
	}
}

// WithIDGenerator 는 session ID 와 JWT ID 에 사용할 ID generator 를 설정한다.
func WithIDGenerator(generator stringGenerator) Option {
	return func(cfg *Config) {
		if generator != nil {
			cfg.idGenerator = generator
		}
	}
}

// Service 는 token 발급, token 검증, refresh-token 교환을 소유한다.
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

// SessionRequest 는 로컬 데모 session 발급 요청이다.
type SessionRequest struct {
	Subject    string   `json:"subject"`
	Role       string   `json:"role"`
	Scopes     []string `json:"scopes"`
	TTLSeconds int      `json:"ttl_seconds"`
}

// SessionResponse 는 발급된 access/refresh token 쌍이다.
type SessionResponse struct {
	TokenType               string `json:"token_type"`
	AccessExpiresInSeconds  int    `json:"access_expires_in_seconds"`
	RefreshExpiresInSeconds int    `json:"refresh_expires_in_seconds"`
	AccessToken             string `json:"access_token"`
	RefreshToken            string `json:"refresh_token"`
	SessionID               string `json:"session_id"`
}

// RefreshRequest 는 refresh token 을 새 access token 으로 교환한다.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// AccessTokenResponse 는 refresh endpoint 응답이다.
type AccessTokenResponse struct {
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	AccessToken      string `json:"access_token"`
	SessionID        string `json:"session_id"`
}

// ProfileResponse 는 검증된 claims 로부터 만든 보호 리소스 표현이다.
type ProfileResponse struct {
	Subject                string `json:"subject"`
	Role                   string `json:"role"`
	Scope                  string `json:"scope"`
	SessionID              string `json:"session_id"`
	AccessExpiresInSeconds int    `json:"access_expires_in_seconds"`
}

// ErrorResponse 는 안정적으로 공개되는 오류 응답 형태다.
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

// NewService 는 결정적인 로컬 JWT 기본값으로 데모 서비스를 구성한다.
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

// IssueSession 은 로컬 데모 access/refresh token 쌍에 서명한다.
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

// ValidateAccess 는 bearer access token 을 검증하고 안정적인 claim context 를 반환한다.
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

// RefreshAccess 는 refresh token 을 검증하고 새 access token 을 반환한다.
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

// NewRouter 는 token refresh claims API용 Gin 라우터를 생성한다.
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
