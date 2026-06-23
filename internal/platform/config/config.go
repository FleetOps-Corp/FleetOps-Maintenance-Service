package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
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

	// Worker Pool (Bulkhead)
	MaxWorkers             int
	WorkerPollIntervalSecs int

	// Preventive Maintenance
	CronIntervalDays        int
	PreventiveKmThreshold   float64
	PreventiveDaysThreshold int

	// External Services
	VehiclesServiceURL    string
	HTTPClientTimeoutSecs int

	// Observability
	MetricsEnabled bool
}

// Load reads configuration from environment variables.
// It returns an error if any required variable is missing or invalid.
func Load() (*Config, error) {
	cfg := &Config{}

	cfg.ServerPort = getEnvOrDefault("SERVER_PORT", "8080")
	cfg.LogLevel = getEnvOrDefault("LOG_LEVEL", "info")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("required environment variable DATABASE_URL is not set")
	}
	cfg.DatabaseURL = dbURL

	maxConns, err := getEnvAsInt("DATABASE_MAX_CONNS", 10)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_MAX_CONNS: %w", err)
	}

	if maxConns < 0 || maxConns > math.MaxInt32 {
		return nil, fmt.Errorf("invalid maxConns: %d (out of int32 range)", maxConns)
	}

	cfg.DatabaseMaxConns = int32(maxConns)

	cfg.MaxWorkers, err = getEnvAsInt("MAX_WORKERS", 5)
	if err != nil {
		return nil, fmt.Errorf("MAX_WORKERS: %w", err)
	}

	cfg.WorkerPollIntervalSecs, err = getEnvAsInt("WORKER_POLL_INTERVAL_SECONDS", 30)
	if err != nil {
		return nil, fmt.Errorf("WORKER_POLL_INTERVAL_SECONDS: %w", err)
	}

	cfg.CronIntervalDays, err = getEnvAsInt("CRON_INTERVAL_DAYS", 7)
	if err != nil {
		return nil, fmt.Errorf("CRON_INTERVAL_DAYS: %w", err)
	}

	kmThresh, err := getEnvAsFloat("PREVENTIVE_KM_THRESHOLD", 10000)
	if err != nil {
		return nil, fmt.Errorf("PREVENTIVE_KM_THRESHOLD: %w", err)
	}
	cfg.PreventiveKmThreshold = kmThresh

	cfg.PreventiveDaysThreshold, err = getEnvAsInt("PREVENTIVE_DAYS_THRESHOLD", 90)
	if err != nil {
		return nil, fmt.Errorf("PREVENTIVE_DAYS_THRESHOLD: %w", err)
	}

	cfg.VehiclesServiceURL = getEnvOrDefault("VEHICLES_SERVICE_URL", "http://api-gateway:8000/api/v1/vehiculos")

	cfg.HTTPClientTimeoutSecs, err = getEnvAsInt("HTTP_CLIENT_TIMEOUT_SECONDS", 10)
	if err != nil {
		return nil, fmt.Errorf("HTTP_CLIENT_TIMEOUT_SECONDS: %w", err)
	}

	cfg.MetricsEnabled = getEnvOrDefault("METRICS_ENABLED", "true") == "true"

	return cfg, nil
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
