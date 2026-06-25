package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/fleetops/maintenance/internal/domain"
	"github.com/fleetops/maintenance/internal/port"
)

// WorkerPool processes queued maintenance records concurrently using goroutines,
// limited by a semaphore channel implementing the Bulkhead pattern.
//
// SAD Reference: ADR-5 — "Patrón Bulkhead: aislamiento de workers evita
// propagación de fallos entre tareas concurrentes"
// Pattern: Bulkhead (Resilience), Worker Pool (Concurrency)
type WorkerPool struct {
	repo          port.MaintenanceRepository
	vehicleClient port.VehicleClient
	maxWorkers    int
	pollInterval  time.Duration
	logger        *slog.Logger
	stopCh        chan struct{}
	stopped       sync.Once
}

// NewWorkerPool constructs a WorkerPool with the given concurrency limit (Bulkhead).
//
// Pattern: Dependency Injection (ADR-7)
// The maxWorkers parameter implements the Bulkhead N value (env: MAX_WORKERS).
func NewWorkerPool(
	repo port.MaintenanceRepository,
	vehicleClient port.VehicleClient,
	maxWorkers int,
	pollIntervalSecs int,
	logger *slog.Logger,
) *WorkerPool {
	return &WorkerPool{
		repo:          repo,
		vehicleClient: vehicleClient,
		maxWorkers:    maxWorkers,
		pollInterval:  time.Duration(pollIntervalSecs) * time.Second,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the worker pool's polling loop. It periodically fetches queued
// maintenances and processes them concurrently up to maxWorkers goroutines.
//
// SAD Reference: "Procesamiento concurrente mediante workers (goroutines)"
func (wp *WorkerPool) Start(ctx context.Context) {
	wp.logger.Info(
		"worker pool started",
		slog.Int("max_workers", wp.maxWorkers),
		slog.Duration("poll_interval", wp.pollInterval),
	)

	go func() {
		ticker := time.NewTicker(wp.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				wp.processQueue(ctx)
			case <-wp.stopCh:
				wp.logger.Info("worker pool stopped")
				return
			case <-ctx.Done():
				wp.logger.Info("worker pool context cancelled")
				return
			}
		}
	}()
}

// Stop signals the worker pool to stop processing.
func (wp *WorkerPool) Stop() {
	wp.stopped.Do(func() {
		close(wp.stopCh)
	})
}

// processQueue fetches queued maintenances and processes them concurrently.
// The semaphore channel enforces the Bulkhead limit (maxWorkers).
func (wp *WorkerPool) processQueue(ctx context.Context) {
	queued, err := wp.repo.ListByStatus(ctx, "queued")
	if err != nil {
		wp.logger.ErrorContext(
			ctx, "worker pool failed to fetch queued items",
			slog.String("error", err.Error()),
		)
		return
	}

	if len(queued) == 0 {
		return
	}

	wp.logger.InfoContext(
		ctx, "worker pool processing queue",
		slog.Int("queued_count", len(queued)),
	)

	semaphore := make(chan struct{}, wp.maxWorkers)
	var wg sync.WaitGroup

	for _, m := range queued {
		m := m
		semaphore <- struct{}{}
		wg.Add(1)

		go func() {
			defer wg.Done()
			defer func() { <-semaphore }()

			wp.processMaintenance(ctx, m)
		}()
	}

	wg.Wait()
}

func (wp *WorkerPool) processMaintenance(ctx context.Context, m *domain.Maintenance) {
	if err := wp.transitionToInProgress(ctx, m); err != nil {
		return
	}

	wp.logger.InfoContext(
		ctx, "processing maintenance",
		slog.String("maintenance_id", m.ID.String()),
		slog.String("type", string(m.Type)),
	)

	if err := wp.transitionToCompleted(ctx, m); err != nil {
		return
	}

	if err := wp.vehicleClient.UpdateVehicleMaintenanceStatus(ctx, m.VehicleID, 0); err != nil {
		wp.logger.WarnContext(
			ctx, "failed to update vehicle maintenance status",
			slog.String("maintenance_id", m.ID.String()),
			slog.String("vehicle_id", m.VehicleID.String()),
			slog.String("error", err.Error()),
		)
	}
}

func (wp *WorkerPool) transitionToInProgress(ctx context.Context, m *domain.Maintenance) error {
	if err := m.MarkInProgress(); err != nil {
		wp.logger.WarnContext(
			ctx, "failed to mark maintenance in_progress",
			slog.String("maintenance_id", m.ID.String()),
			slog.String("error", err.Error()),
		)
		return err
	}

	if err := wp.repo.UpdateStatus(ctx, m); err != nil {
		wp.logger.ErrorContext(
			ctx, "failed to persist in_progress status",
			slog.String("maintenance_id", m.ID.String()),
			slog.String("error", err.Error()),
		)
		return err
	}

	return nil
}

func (wp *WorkerPool) transitionToCompleted(ctx context.Context, m *domain.Maintenance) error {
	if err := m.MarkCompleted(); err != nil {
		wp.logger.ErrorContext(
			ctx, "failed to mark maintenance completed",
			slog.String("maintenance_id", m.ID.String()),
			slog.String("error", err.Error()),
		)
		return err
	}

	if err := wp.repo.UpdateStatus(ctx, m); err != nil {
		wp.logger.ErrorContext(
			ctx, "failed to persist completed status",
			slog.String("maintenance_id", m.ID.String()),
			slog.String("error", err.Error()),
		)
		return err
	}

	return nil
}
