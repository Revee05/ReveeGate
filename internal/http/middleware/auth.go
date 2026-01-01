package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/reveegate/reveegate/internal/config"
	redisRepo "github.com/reveegate/reveegate/internal/repository/redis"
)

// ClaimsContextKey is the context key for JWT claims
type ClaimsContextKey struct{}

// Claims represents JWT claims
type Claims struct {
	UserID  string `json:"user_id"`
	Subject string `json:"sub"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

// Auth middleware provides JWT authentication
type Auth struct {
	config config.JWTConfig
	cache  *redisRepo.Cache
	logger *slog.Logger
}

// NewAuth creates a new Auth middleware
func NewAuth(cfg config.JWTConfig, cache *redisRepo.Cache, logger *slog.Logger) *Auth {
	return &Auth{
		config: cfg,
		cache:  cache,
		logger: logger,
	}
}

// GenerateTokens generates access and refresh tokens
func (a *Auth) GenerateTokens(userID, role string) (accessToken, refreshToken string, err error) {
	now := time.Now()

	// Generate access token
	accessClaims := &Claims{
		UserID:  userID,
		Subject: userID,
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(a.config.AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "reveegate",
			Subject:   userID,
		},
	}

	accessTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = accessTokenObj.SignedString([]byte(a.config.Secret))
	if err != nil {
		return "", "", err
	}

	// Generate refresh token
	refreshClaims := &Claims{
		UserID:  userID,
		Subject: userID,
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(a.config.RefreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "reveegate",
			Subject:   userID,
		},
	}

	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshTokenObj.SignedString([]byte(a.config.Secret))
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// ValidateToken validates a JWT token and returns the claims
func (a *Auth) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(a.config.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// ValidateOverlayToken validates an overlay token
func (a *Auth) ValidateOverlayToken(ctx context.Context, token string) (bool, error) {
	// Check if token exists in cache or database
	// For now, accept any non-empty UUID-like token
	if len(token) >= 32 {
		return true, nil
	}
	return false, errors.New("invalid overlay token")
}

// HashToken hashes a token for storage
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Middleware returns the JWT authentication middleware
func (a *Auth) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing authorization header", http.StatusUnauthorized)
				return
			}

			// Check Bearer prefix
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// Validate token
			claims, err := a.ValidateToken(tokenString)
			if err != nil {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Add claims to context
			ctx := context.WithValue(r.Context(), ClaimsContextKey{}, claims)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetClaims gets the claims from context
func GetClaims(ctx context.Context) *Claims {
	if claims, ok := ctx.Value(ClaimsContextKey{}).(*Claims); ok {
		return claims
	}
	return nil
}
