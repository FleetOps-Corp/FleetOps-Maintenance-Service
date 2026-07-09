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
	"github.com/fleetops/maintenance/internal/domain"
	"github.com/fleetops/maintenance/internal/handler"
	"github.com/fleetops/maintenance/internal/platform/config"
	"github.com/fleetops/maintenance/internal/platform/database"
	"github.com/fleetops/maintenance/internal/platform/logger"
	"github.com/fleetops/maintenance/internal/port"
	"github.com/fleetops/maintenance/internal/service"
)

func newSQSClient(ctx context.Context, region string, log *slog.Logger) *sqs.Client {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		log.Error("failed to load AWS config for SQS", slog.String("error", err.Error()))
		os.Exit(1)
	}
	return sqs.NewFromConfig(awsCfg)
}

func newEventPublisher(
	client *sqs.Client,
	queueURL string,
	useMockFallback bool,
	log *slog.Logger,
) port.EventPublisher {
	if client == nil || queueURL == "" {
		log.Warn("SQS_VEHICLES_URL is not set, using NOOP Event Publisher")
		return &noopPublisher{log: log}
	}
	return messaging.NewSQSPublisher(client, queueURL, useMockFallback, log)
}

func startIncidentConsumer(
	ctx context.Context,
	client *sqs.Client,
	queueURL string,
	svc *service.CorrectiveMaintenanceService,
	log *slog.Logger,
) func() {
	if client == nil || queueURL == "" {
		return func() {
			// No-op: SQS Consumer is disabled because SQS Incidents URL is not set.
		}
	}
	consumer := messaging.NewSQSConsumer(client, queueURL, svc, log)
	consumer.Start(ctx)
	return consumer.Stop
}

func runServer(server *http.Server, log *slog.Logger) {
	go func() {
		log.Info("HTTP server listening", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()
}

func shutdown(server *http.Server, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("server forced shutdown", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

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
	// =========================================================================

	// Data Access Layer
	maintenanceRepo := repository.NewPostgresMaintenanceRepository(pool)
	vehicleClient := client.NewHTTPVehicleClient(cfg.VehiclesServiceURL, cfg.VehiclesAPIToken, cfg.HTTPClientTimeoutSecs, cfg.UseMockFallback, log)

	// Messaging
	var sqsIncidentsClient *sqs.Client
	if cfg.SQSQueueIncidentsURL != "" {
		sqsIncidentsClient = newSQSClient(ctx, cfg.AWSRegionIncidents, log)
	}

	var sqsVehiclesClient *sqs.Client
	if cfg.SQSQueueVehiclesURL != "" {
		sqsVehiclesClient = newSQSClient(ctx, cfg.AWSRegionVehicles, log)
	}

	eventPublisher := newEventPublisher(sqsVehiclesClient, cfg.SQSQueueVehiclesURL, cfg.UseMockFallback, log)

	// Business Logic Layer
	correctiveSvc := service.NewCorrectiveMaintenanceService(maintenanceRepo, eventPublisher, log)
	preventiveSvc := service.NewPreventiveMaintenanceService(
		maintenanceRepo, vehicleClient, eventPublisher,
		cfg.PreventiveKmThresholds, cfg.PreventiveDaysThreshold,
		cfg.CronIntervalMinutes, log,
	)
	queueSvc := service.NewQueueService(maintenanceRepo, eventPublisher, log)
	workerPool := service.NewWorkerPool(
		maintenanceRepo,
		cfg.MaxWorkers, cfg.WorkerPollIntervalSecs, cfg.UseMockFallback, log,
	)

	releaseSvc := service.NewReleaseService(maintenanceRepo, eventPublisher, log, cfg.ReleaseMinutesThreshold)
	releaseWorker := handler.NewReleaseWorker(releaseSvc, log, cfg.ReleasePollIntervalSecs)

	// Presentation Layer
	maintenanceHandler := handler.NewMaintenanceHandler(correctiveSvc, queueSvc, log)
	healthHandler := handler.NewHealthHandler(pool)
	router := handler.NewRouter(maintenanceHandler, healthHandler, log, cfg.MetricsEnabled, cfg.JWTPublicKey, cfg.JWTAlgorithm)

	// =========================================================================
	// Start background services
	// =========================================================================

	preventiveSvc.Start(ctx)
	defer preventiveSvc.Stop()

	workerPool.Start(ctx)
	defer workerPool.Stop()

	releaseWorker.Start(ctx)
	defer releaseWorker.Stop()

	stopConsumer := startIncidentConsumer(ctx, sqsIncidentsClient, cfg.SQSQueueIncidentsURL, correctiveSvc, log)
	defer stopConsumer()

	// USE_MOCK_FALLBACK: Auto-generate corrective maintenance event for testing
	if cfg.UseMockFallback {
		go func() {
			time.Sleep(5 * time.Second)
			log.Info("USE_MOCK_FALLBACK: Automatically generating a corrective maintenance event for testing")
			_, err := correctiveSvc.CreateCorrective(context.Background(), "ABC-123", "INC-MOCK-999", 8)
			if err != nil {
				log.Error("failed to auto-generate corrective maintenance fallback", slog.String("error", err.Error()))
			}
		}()
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

	runServer(server, log)

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutdown signal received, draining connections...")
	shutdown(server, log)

	log.Info("FleetOps Maintenance Microservice stopped gracefully")
}

// noopPublisher is a fallback publisher if SQS is not configured
type noopPublisher struct {
	log *slog.Logger
}

func (n *noopPublisher) PublishMaintenanceEvent(ctx context.Context, m *domain.Maintenance, status string) error {
	n.log.InfoContext(ctx, "NOOP publish event", slog.String("id", m.ID.String()), slog.String("status", status))
	return nil
}
