package config

import (
	"crypto/rsa"
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

// Config holds all externalized configuration for the maintenance microservice.
// All values are loaded from environment variables following the Externalized
// Configuration pattern.
//
// Pattern: Externalized Configuration
// SAD Reference: ADR (Variabilidad) — "configuraciones en archivos de texto
// independientes de las clases que las ejecutan"
type Config struct {
	// Server
	ServerPort string
	LogLevel   string

	// Database
	DatabaseURL      string
	DatabaseMaxConns int32

	// Domain & Business Rules
	MaxWorkers             int
	WorkerPollIntervalSecs int
	CronIntervalDays       int
	PreventiveKmThreshold  float64
	PreventiveDaysThreshold int

	// Integration Clients
	VehiclesServiceURL    string
	VehiclesAPIToken      string
	HTTPClientTimeoutSecs int

	// Observability
	MetricsEnabled bool

	// Messaging
	SQSQueueURL string
	AWSRegion   string

	// JWT Authentication
	JWTAlgorithm     string
	JWTPublicKeyPath string
	JWTPublicKey     *rsa.PublicKey
}

// Load reads configuration from environment variables.
// It returns an error if any required variable is missing or invalid.
func Load() (*Config, error) {
	_ = godotenv.Load() // silently load .env if it exists

	cfg := &Config{}

	cfg.ServerPort = getEnvOrDefault("SERVER_PORT", "8080")
	cfg.LogLevel = getEnvOrDefault("LOG_LEVEL", "info")

	if err := loadDatabaseConfig(cfg); err != nil {
		return nil, err
	}

	if err := loadThresholdConfig(cfg); err != nil {
		return nil, err
	}

	cfg.VehiclesServiceURL = getEnvOrDefault("VEHICLES_SERVICE_URL", "http://api-gateway:8000")
	cfg.VehiclesAPIToken = getEnvOrDefault("VEHICLES_API_TOKEN", "")

	var err error
	cfg.HTTPClientTimeoutSecs, err = getEnvAsInt("HTTP_CLIENT_TIMEOUT_SECONDS", 10)
	if err != nil {
		return nil, fmt.Errorf("HTTP_CLIENT_TIMEOUT_SECONDS: %w", err)
	}

	cfg.MetricsEnabled = getEnvOrDefault("METRICS_ENABLED", "true") == "true"
	cfg.SQSQueueURL = os.Getenv("SQS_QUEUE_URL")
	cfg.AWSRegion = getEnvOrDefault("AWS_REGION", "us-east-1")

	if err := loadJWTConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadDatabaseConfig(cfg *Config) error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("required environment variable DATABASE_URL is not set")
	}
	cfg.DatabaseURL = dbURL

	maxConns, err := getEnvAsInt("DATABASE_MAX_CONNS", 10)
	if err != nil {
		return fmt.Errorf("DATABASE_MAX_CONNS: %w", err)
	}

	if maxConns < 0 || maxConns > math.MaxInt32 {
		return fmt.Errorf("invalid maxConns: %d (out of int32 range)", maxConns)
	}

	cfg.DatabaseMaxConns = int32(maxConns)
	return nil
}

func loadThresholdConfig(cfg *Config) error {
	var err error
	cfg.MaxWorkers, err = getEnvAsInt("MAX_WORKERS", 5)
	if err != nil {
		return fmt.Errorf("MAX_WORKERS: %w", err)
	}

	cfg.WorkerPollIntervalSecs, err = getEnvAsInt("WORKER_POLL_INTERVAL_SECONDS", 30)
	if err != nil {
		return fmt.Errorf("WORKER_POLL_INTERVAL_SECONDS: %w", err)
	}

	cfg.CronIntervalDays, err = getEnvAsInt("CRON_INTERVAL_DAYS", 7)
	if err != nil {
		return fmt.Errorf("CRON_INTERVAL_DAYS: %w", err)
	}

	kmThresh, err := getEnvAsFloat("PREVENTIVE_KM_THRESHOLD", 10000)
	if err != nil {
		return fmt.Errorf("PREVENTIVE_KM_THRESHOLD: %w", err)
	}
	cfg.PreventiveKmThreshold = kmThresh

	cfg.PreventiveDaysThreshold, err = getEnvAsInt("PREVENTIVE_DAYS_THRESHOLD", 90)
	if err != nil {
		return fmt.Errorf("PREVENTIVE_DAYS_THRESHOLD: %w", err)
	}

	return nil
}

func loadJWTConfig(cfg *Config) error {
	cfg.JWTAlgorithm = getEnvOrDefault("JWT_ALGORITHM", "RS256")
	cfg.JWTPublicKeyPath = getEnvOrDefault("JWT_PUBLIC_KEY_PATH", "/run/secrets/jwt_public.pem")

	// Validate algorithm up-front (only RSA asymmetric signing algorithms are supported for public key verification)
	if cfg.JWTAlgorithm != "RS256" && cfg.JWTAlgorithm != "RS384" && cfg.JWTAlgorithm != "RS512" {
		return fmt.Errorf("invalid JWT_ALGORITHM %q: only RS256, RS384, and RS512 are supported for public key signature verification", cfg.JWTAlgorithm)
	}

	path := cfg.JWTPublicKeyPath
	// Fallback for local development if running outside container
	if _, err := os.Stat(path); os.IsNotExist(err) && path == "/run/secrets/jwt_public.pem" {
		localFallback := "./secrets/jwt_public.pem"
		if _, errSub := os.Stat(localFallback); errSub == nil {
			path = localFallback
		}
	}
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read JWT public key file from path %q (resolved: %q): %w", cfg.JWTPublicKeyPath, path, err)
	}
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(keyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse JWT public key: %w", err)
	}
	cfg.JWTPublicKey = pubKey
	return nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) (int, error) {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal, nil
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value %q: %w", valStr, err)
	}
	return val, nil
}

func getEnvAsFloat(key string, defaultVal float64) (float64, error) {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal, nil
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float value %q: %w", valStr, err)
	}
	return val, nil
}
