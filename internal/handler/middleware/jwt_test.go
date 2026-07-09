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

type jwtTestCase struct {
	name           string
	authHeader     string
	setupToken     func() string
	expectedStatus int
	expectedSub    string
	expectedRole   string
	expectedError  string
	expectedMsg    string
}

func TestJWTAuth(t *testing.T) {
	// Generate key pair for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	// Generate another key pair for invalid signature testing
	otherPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create logger that discards output to avoid polluting test logs
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Helper function to create valid token string
	createToken := func(key *rsa.PrivateKey, claims jwt.MapClaims, method jwt.SigningMethod) string {
		token := jwt.NewWithClaims(method, claims)
		tokenString, errSign := token.SignedString(key)
		require.NoError(t, errSign)
		return tokenString
	}

	tests := []jwtTestCase{
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
			expectedSub:    "user-123",
			expectedRole:   "admin",
		},
		{
			name: "valid token with multiple spaces",
			setupToken: func() string {
				return "Bearer    " + createToken(privateKey, jwt.MapClaims{
					"sub":  "user-123",
					"role": "admin",
					"exp":  time.Now().Add(1 * time.Hour).Unix(),
				}, jwt.SigningMethodRS256)
			},
			expectedStatus: http.StatusOK,
			expectedSub:    "user-123",
			expectedRole:   "admin",
		},
		{
			name:           "missing authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "unauthorized",
			expectedMsg:    "Missing authentication token.",
		},
		{
			name:           "malformed authorization header",
			authHeader:     "Bearer-Token",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "invalid_token_format",
			expectedMsg:    "invalid authentication token format: expected Bearer token",
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
			expectedError:  "token_expired",
			expectedMsg:    "Token has expired. Please log in again.",
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
			expectedError:  "invalid_token",
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
			expectedError:  "invalid_token",
		},
	}

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

	mw := middleware.JWTAuth(publicKey, "RS256", false, logger)
	handlerToTest := mw(dummyHandler)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executeTestScenario(t, tt, handlerToTest)
		})
	}
}

func executeTestScenario(t *testing.T, tt jwtTestCase, handlerToTest http.Handler) {
	req := httptest.NewRequest("GET", "/test", nil)
	if tt.setupToken != nil {
		req.Header.Set("Authorization", tt.setupToken())
	} else if tt.authHeader != "" {
		req.Header.Set("Authorization", tt.authHeader)
	}

	rr := httptest.NewRecorder()
	handlerToTest.ServeHTTP(rr, req)

	require.Equal(t, tt.expectedStatus, rr.Code)

	if tt.expectedStatus == http.StatusOK {
		var respClaims jwt.MapClaims
		errDec := json.NewDecoder(rr.Body).Decode(&respClaims)
		require.NoError(t, errDec)
		require.Equal(t, tt.expectedSub, respClaims["sub"])
		require.Equal(t, tt.expectedRole, respClaims["role"])
	} else {
		var errResp dto.ErrorResponse
		errDec := json.NewDecoder(rr.Body).Decode(&errResp)
		require.NoError(t, errDec)
		require.Equal(t, tt.expectedError, errResp.Error)
		if tt.expectedMsg != "" {
			require.Equal(t, tt.expectedMsg, errResp.Message)
		}
		require.Equal(t, tt.expectedStatus, errResp.Code)
		if tt.expectedError == "token_expired" {
			require.Equal(t, "Bearer", rr.Header().Get("WWW-Authenticate"))
		}
	}
}
