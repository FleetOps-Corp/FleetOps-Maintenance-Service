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

	// Generate another key pair for invalid signature testing
	otherPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create logger that discards output to avoid polluting test output
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Helper function to create valid token string
	createToken := func(key *rsa.PrivateKey, claims jwt.MapClaims, method jwt.SigningMethod) string {
		token := jwt.NewWithClaims(method, claims)
		tokenString, errSign := token.SignedString(key)
		require.NoError(t, errSign)
		return tokenString
	}

	tests := []struct {
		name           string
		authHeader     string
		setupToken     func() string
		expectedStatus int
		verifyResponse func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "valid token",
			setupToken: func() string {
				return "Bearer " + createToken(privateKey, jwt.MapClaims{
					"sub":  "user-123",
					"role": "admin",
					"exp":  time.Now().Add(1 * time.Hour).Unix(),
				}, jwt.SigningMethodRS256)
			},
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var respClaims jwt.MapClaims
				errDec := json.NewDecoder(rec.Body).Decode(&respClaims)
				require.NoError(t, errDec)
				require.Equal(t, "user-123", respClaims["sub"])
				require.Equal(t, "admin", respClaims["role"])
			},
		},
		{
			name:           "missing authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var errResp dto.ErrorResponse
				errDec := json.NewDecoder(rec.Body).Decode(&errResp)
				require.NoError(t, errDec)
				require.Equal(t, "unauthorized", errResp.Error)
				require.Equal(t, "Missing authentication token.", errResp.Message)
				require.Equal(t, http.StatusUnauthorized, errResp.Code)
			},
		},
		{
			name:           "malformed authorization header",
			authHeader:     "Bearer-Token",
			expectedStatus: http.StatusUnauthorized,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var errResp dto.ErrorResponse
				errDec := json.NewDecoder(rec.Body).Decode(&errResp)
				require.NoError(t, errDec)
				require.Equal(t, "invalid_token_format", errResp.Error)
				require.Equal(t, "Invalid authentication token format. Use Bearer <token>.", errResp.Message)
			},
		},
		{
			name: "expired token",
			setupToken: func() string {
				return "Bearer " + createToken(privateKey, jwt.MapClaims{
					"sub": "user-123",
					"exp": time.Now().Add(-1 * time.Hour).Unix(),
				}, jwt.SigningMethodRS256)
			},
			expectedStatus: http.StatusUnauthorized,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				require.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
				var errResp dto.ErrorResponse
				errDec := json.NewDecoder(rec.Body).Decode(&errResp)
				require.NoError(t, errDec)
				require.Equal(t, "token_expired", errResp.Error)
				require.Equal(t, "Token has expired. Please log in again.", errResp.Message)
			},
		},
		{
			name: "invalid signature",
			setupToken: func() string {
				return "Bearer " + createToken(otherPrivateKey, jwt.MapClaims{
					"sub": "user-123",
					"exp": time.Now().Add(1 * time.Hour).Unix(),
				}, jwt.SigningMethodRS256)
			},
			expectedStatus: http.StatusUnauthorized,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var errResp dto.ErrorResponse
				errDec := json.NewDecoder(rec.Body).Decode(&errResp)
				require.NoError(t, errDec)
				require.Equal(t, "invalid_token", errResp.Error)
			},
		},
		{
			name: "unexpected algorithm",
			setupToken: func() string {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"sub": "user-123",
					"exp": time.Now().Add(1 * time.Hour).Unix(),
				})
				tokenString, errSign := token.SignedString([]byte("some-secret"))
				require.NoError(t, errSign)
				return "Bearer " + tokenString
			},
			expectedStatus: http.StatusUnauthorized,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var errResp dto.ErrorResponse
				errDec := json.NewDecoder(rec.Body).Decode(&errResp)
				require.NoError(t, errDec)
				require.Equal(t, "invalid_token", errResp.Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				claims, ok := middleware.ClaimsFromContext(r.Context())
				if !ok {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(claims)
			})

			mw := middleware.JWTAuth(publicKey, "RS256", logger)
			handlerToTest := mw(dummyHandler)

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.setupToken != nil {
				req.Header.Set("Authorization", tt.setupToken())
			} else {
				if tt.authHeader != "" {
					req.Header.Set("Authorization", tt.authHeader)
				}
			}

			rr := httptest.NewRecorder()
			handlerToTest.ServeHTTP(rr, req)

			require.Equal(t, tt.expectedStatus, rr.Code)
			if tt.verifyResponse != nil {
				tt.verifyResponse(t, rr)
			}
		})
	}
}
