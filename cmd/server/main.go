// Package main is the application entry point and Composition Root.
// It wires all dependencies together following the Dependency Injection
// pattern and starts the HTTP server.
//
// Pattern: Composition Root
// SAD Reference: ADR-7 — "Dependency Injection: las dependencias son
// abstraídas mediante interfaces"
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/fleetops/maintenance/internal/adapter/client"
	"github.com/fleetops/maintenance/internal/adapter/messaging"
	"github.com/fleetops/maintenance/internal/adapter/repository"
	"github.com/fleetops/maintenance/internal/handler"
	"github.com/fleetops/maintenance/internal/platform/config"
	"github.com/fleetops/maintenance/internal/platform/database"
	"github.com/fleetops/maintenance/internal/platform/logger"
	"github.com/fleetops/maintenance/internal/service"
)

func main() {
	// Load configuration from environment variables
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	log := logger.New(cfg.LogLevel)
	slog.SetDefault(log)

	log.Info(
		"starting FleetOps Maintenance Microservice",
		slog.String("port", cfg.ServerPort),
		slog.String("log_level", cfg.LogLevel),
	)

	// Initialize database connection pool
	ctx := context.Background()
	pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		log.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()
	log.Info("database connection established")

	// =========================================================================
	// Dependency Injection — Composition Root
	// Wire: Adapters → Ports ← Services → Handlers
	// =========================================================================

	// Data Access Layer: Adapters (implement Port interfaces)
	maintenanceRepo := repository.NewPostgresMaintenanceRepository(pool)
	vehicleClient := client.NewHTTPVehicleClient(cfg.VehiclesServiceURL, cfg.HTTPClientTimeoutSecs)

	// Business Logic Layer: Services (depend on Port interfaces)
	correctiveSvc := service.NewCorrectiveMaintenanceService(maintenanceRepo, log)
	preventiveSvc := service.NewPreventiveMaintenanceService(
		maintenanceRepo, vehicleClient,
		cfg.PreventiveKmThreshold, cfg.PreventiveDaysThreshold,
		cfg.CronIntervalDays, log,
	)
	queueSvc := service.NewQueueService(maintenanceRepo, log)
	workerPool := service.NewWorkerPool(
		maintenanceRepo, vehicleClient,
		cfg.MaxWorkers, cfg.WorkerPollIntervalSecs, log,
	)

	// Presentation Layer: Handlers (depend on Services)
	maintenanceHandler := handler.NewMaintenanceHandler(correctiveSvc, queueSvc, log)
	healthHandler := handler.NewHealthHandler(pool)

	// Router
	router := handler.NewRouter(maintenanceHandler, healthHandler, log, cfg.MetricsEnabled)

	// =========================================================================
	// Start background services
	// =========================================================================

	// Start preventive maintenance scheduler (Cron Handler)
	preventiveSvc.Start(ctx)
	defer preventiveSvc.Stop()

	// Start worker pool (Bulkhead pattern)
	workerPool.Start(ctx)
	defer workerPool.Stop()

	// =========================================================================
	// Messaging: SQS Consumer
	// =========================================================================
	if cfg.SQSQueueURL != "" {
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
		if err != nil {
			log.Error("failed to load AWS config for SQS", slog.String("error", err.Error()))
			os.Exit(1)
		}
		sqsClient := sqs.NewFromConfig(awsCfg)
		sqsConsumer := messaging.NewSQSConsumer(sqsClient, cfg.SQSQueueURL, correctiveSvc, log)
		sqsConsumer.Start(ctx)
		defer sqsConsumer.Stop()
	} else {
		log.Warn("SQS_QUEUE_URL is not set, SQS consumer will NOT start")
	}

	// =========================================================================
	// HTTP Server with graceful shutdown
	// =========================================================================

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("HTTP server listening", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutdown signal received, draining connections...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced shutdown", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("FleetOps Maintenance Microservice stopped gracefully")
}
