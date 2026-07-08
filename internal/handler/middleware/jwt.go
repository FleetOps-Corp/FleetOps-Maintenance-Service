package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fleetops/maintenance/internal/handler/dto"
)

// contextKey is an unexported empty struct to avoid key collisions in context.
type contextKey struct{}

var userClaimsKey = contextKey{}

// ClaimsFromContext safely extracts the parsed JWT claims from the request context.
func ClaimsFromContext(ctx context.Context) (jwt.MapClaims, bool) {
	claims, ok := ctx.Value(userClaimsKey).(jwt.MapClaims)
	return claims, ok
}

// JWTAuth returns a middleware that validates JWT tokens using an RSA public key.
func JWTAuth(publicKey *rsa.PublicKey, algorithm string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr, err := extractToken(r)
			if err != nil {
				logger.Warn("JWT validation failed: header extraction error", slog.String("error", err.Error()))
				respondError(w, http.StatusUnauthorized, "invalid_token_format", err.Error(), logger)
				return
			}
			if tokenStr == "" {
				logger.Warn("JWT validation failed: missing Authorization header")
				respondError(w, http.StatusUnauthorized, "unauthorized", "Missing authentication token.", logger)
				return
			}

			claims, err := parseAndValidateToken(tokenStr, publicKey, algorithm)
			if err != nil {
				if errors.Is(err, jwt.ErrTokenExpired) {
					logger.Warn("JWT validation failed: token expired")
					respondError(w, http.StatusUnauthorized, "token_expired", "Token has expired. Please log in again.", logger)
					return
				}
				logger.Warn("JWT validation failed", slog.String("error", err.Error()))
				respondError(w, http.StatusUnauthorized, "invalid_token", "Invalid authentication token.", logger)
				return
			}

			// Add claims to context so handlers can access user info if needed
			ctx := context.WithValue(r.Context(), userClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractToken extracts the raw token string from the Authorization header.
func extractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", nil
	}

	fields := strings.Fields(authHeader)
	if len(fields) != 2 || strings.ToLower(fields[0]) != "bearer" {
		return "", errors.New("invalid authentication token format: expected Bearer token")
	}

	return fields[1], nil
}

// parseAndValidateToken parses the token string, verifies its signature and algorithm, and returns the claims.
func parseAndValidateToken(tokenStr string, publicKey *rsa.PublicKey, algorithm string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != algorithm {
			return nil, fmt.Errorf("unexpected signing method algorithm: %v", t.Header["alg"])
		}
		return publicKey, nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("token is invalid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("unable to parse claims")
	}

	return claims, nil
}

// respondError writes a structured JSON error response.
//
//nolint:unparam
func respondError(w http.ResponseWriter, code int, errType, message string, logger *slog.Logger) {
	if code == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	errResp := dto.ErrorResponse{
		Error:   errType,
		Message: message,
		Code:    code,
	}
	if err := json.NewEncoder(w).Encode(errResp); err != nil {
		logger.Error("failed to encode auth error response", slog.String("error", err.Error()))
	}
}
