package middleware_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
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

	mw := middleware.JWTAuth(publicKey, "RS256", logger)
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

//nolint:staticcheck // x509 legacy PEM encryption is deprecated but required to test the decryption capability for legacy systems.
func TestEncryptedPrivateKeyDecryption(t *testing.T) {
	passphrase := "super-secreto-de-prueba"

	// 1. GENERAMOS LLAVE DE PRUEBA EN MEMORIA (Para simular el escenario)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Convertimos la llave privada a bytes DER (PKCS#1)
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	// Encriptamos la llave privada en formato PEM usando la contraseña
	// (Simula el contenido de un archivo "jwt_private_encrypted.pem")
	encryptedBlock, err := x509.EncryptPEMBlock(
		rand.Reader,
		"RSA PRIVATE KEY",
		privDER,
		[]byte(passphrase),
		x509.PEMCipherAES256,
	)
	require.NoError(t, err)
	encryptedPEMBytes := pem.EncodeToMemory(encryptedBlock)

	// ----------------------------------------------------------------
	// 2. PRUEBA DE DESENCRIPTACIÓN Y LECTURA (Lo que haría tu servicio)
	// ----------------------------------------------------------------

	// Decodificamos el bloque PEM
	block, rest := pem.Decode(encryptedPEMBytes)
	require.NotNil(t, block, "El bloque PEM no debe ser nil")
	require.Empty(t, rest, "No debería haber contenido residual en el PEM")

	// Validamos si efectivamente viene encriptado
	var decryptedDER []byte
	if x509.IsEncryptedPEMBlock(block) {
		// Desencriptamos el bloque PEM usando la contraseña
		decryptedDER, err = x509.DecryptPEMBlock(block, []byte(passphrase))
		require.NoError(t, err, "Error al desencriptar el bloque PEM de la llave privada")
	} else {
		decryptedDER = block.Bytes
	}

	// Parseamos la llave privada una vez desencriptada
	parsedPrivateKey, err := x509.ParsePKCS1PrivateKey(decryptedDER)
	require.NoError(t, err, "Error al parsear los bytes desencriptados a clave RSA")

	// ----------------------------------------------------------------
	// 3. INTEGRACIÓN CON EL FLUJO DEL MIDDLEWARE
	// ----------------------------------------------------------------

	// Generamos un JWT utilizando la llave privada que acabamos de desencriptar
	claims := jwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(parsedPrivateKey)
	require.NoError(t, err, "Error al firmar el token con la llave privada desencriptada")

	// Usamos la llave pública correspondiente (que no requiere contraseña) para validar
	publicKey := &privateKey.PublicKey
	pubDER, err := x509.MarshalPKIXPublicKey(publicKey)
	require.NoError(t, err)

	pubPEMBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}
	pubPEMBytes := pem.EncodeToMemory(pubPEMBlock)

	// Parseamos la llave pública de manera estándar
	parsedPublicKey, err := jwt.ParseRSAPublicKeyFromPEM(pubPEMBytes)
	require.NoError(t, err)

	// Validamos que el token firmado por la llave desencriptada sea válido para la llave pública
	parsedToken, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return parsedPublicKey, nil
	})
	require.NoError(t, err)
	require.True(t, parsedToken.Valid)
}
