package middleware_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/fleetops/maintenance/internal/handler/dto"
	"github.com/fleetops/maintenance/internal/handler/middleware"
)

func TestJWTAuth(t *testing.T) {
	// Generate key pair for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	// Create logger that discards output to avoid polluting test logs
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Define a dummy handler to serve as the "next" handler in the chain
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(middleware.UserClaimsKey).(jwt.MapClaims)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(claims)
	})

	// Wrap dummyHandler in our middleware
	mw := middleware.JWTAuth(publicKey, "RS256", logger)
	handlerToTest := mw(dummyHandler)

	t.Run("valid token", func(t *testing.T) {
		// Create claims
		claims := jwt.MapClaims{
			"sub":  "user-123",
			"role": "admin",
			"exp":  time.Now().Add(1 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(privateKey)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		rr := httptest.NewRecorder()

		handlerToTest.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var respClaims jwt.MapClaims
		err = json.NewDecoder(rr.Body).Decode(&respClaims)
		require.NoError(t, err)
		if respClaims["sub"] != "user-123" {
			t.Errorf("expected sub user-123, got %v", respClaims["sub"])
		}
		if respClaims["role"] != "admin" {
			t.Errorf("expected role admin, got %v", respClaims["role"])
		}
	})

	t.Run("missing authorization header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		handlerToTest.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", rr.Code)
		}
		var errResp dto.ErrorResponse
		err := json.NewDecoder(rr.Body).Decode(&errResp)
		require.NoError(t, err)
		if errResp.Error != "unauthorized" {
			t.Errorf("expected error type unauthorized, got %q", errResp.Error)
		}
		if errResp.Message != "Missing authentication token." {
			t.Errorf("expected message Missing authentication token., got %q", errResp.Message)
		}
		if errResp.Code != http.StatusUnauthorized {
			t.Errorf("expected error code 401, got %d", errResp.Code)
		}
	})

	t.Run("malformed authorization header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer-Token")
		rr := httptest.NewRecorder()

		handlerToTest.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", rr.Code)
		}
		var errResp dto.ErrorResponse
		err := json.NewDecoder(rr.Body).Decode(&errResp)
		require.NoError(t, err)
		if errResp.Error != "invalid_token_format" {
			t.Errorf("expected error type invalid_token_format, got %q", errResp.Error)
		}
		if errResp.Message != "Invalid authentication token format. Use Bearer <token>." {
			t.Errorf("expected message Invalid authentication token format. Use Bearer <token>., got %q", errResp.Message)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		claims := jwt.MapClaims{
			"sub": "user-123",
			"exp": time.Now().Add(-1 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(privateKey)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		rr := httptest.NewRecorder()

		handlerToTest.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", rr.Code)
		}
		if rr.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("expected WWW-Authenticate: Bearer header, got %q", rr.Header().Get("WWW-Authenticate"))
		}
		var errResp dto.ErrorResponse
		err = json.NewDecoder(rr.Body).Decode(&errResp)
		require.NoError(t, err)
		if errResp.Error != "token_expired" {
			t.Errorf("expected error type token_expired, got %q", errResp.Error)
		}
		if errResp.Message != "Token has expired. Please log in again." {
			t.Errorf("expected message Token has expired. Please log in again., got %q", errResp.Message)
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		// Generate another private key
		otherPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		claims := jwt.MapClaims{
			"sub": "user-123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(otherPrivateKey)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		rr := httptest.NewRecorder()

		handlerToTest.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", rr.Code)
		}
		var errResp dto.ErrorResponse
		err = json.NewDecoder(rr.Body).Decode(&errResp)
		require.NoError(t, err)
		if errResp.Error != "invalid_token" {
			t.Errorf("expected error type invalid_token, got %q", errResp.Error)
		}
	})

	t.Run("unexpected algorithm", func(t *testing.T) {
		// We sign with HMAC (HS256) but middleware expects RS256
		claims := jwt.MapClaims{
			"sub": "user-123",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte("some-secret"))
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		rr := httptest.NewRecorder()

		handlerToTest.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", rr.Code)
		}
		var errResp dto.ErrorResponse
		err = json.NewDecoder(rr.Body).Decode(&errResp)
		require.NoError(t, err)
		if errResp.Error != "invalid_token" {
			t.Errorf("expected error type invalid_token, got %q", errResp.Error)
		}
	})
}
