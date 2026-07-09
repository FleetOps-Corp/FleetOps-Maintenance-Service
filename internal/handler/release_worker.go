package handler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/fleetops/maintenance/internal/service"
)

// ReleaseWorker periodically triggers the ReleaseService to automatically finalize
// old maintenances and release vehicles via SQS events.
//
// Pattern: Background Worker
type ReleaseWorker struct {
	svc          *service.ReleaseService
	logger       *slog.Logger
	pollInterval time.Duration
	wg           sync.WaitGroup
	quit         chan struct{}
}

// NewReleaseWorker creates a new ReleaseWorker.
func NewReleaseWorker(svc *service.ReleaseService, logger *slog.Logger, pollIntervalSecs int) *ReleaseWorker {
	return &ReleaseWorker{
		svc:          svc,
		logger:       logger,
		pollInterval: time.Duration(pollIntervalSecs) * time.Second,
		quit:         make(chan struct{}),
	}
}

// Start launches the worker in a background goroutine.
func (w *ReleaseWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
}

// Stop gracefully shuts down the worker, waiting for the current cycle to finish.
func (w *ReleaseWorker) Stop() {
	close(w.quit)
	w.wg.Wait()
}

func (w *ReleaseWorker) run(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	w.logger.InfoContext(ctx, "release worker started", slog.Duration("interval", w.pollInterval))

	for {
		select {
		case <-ctx.Done():
			w.logger.InfoContext(ctx, "release worker stopped (context cancelled)")
			return
		case <-w.quit:
			w.logger.InfoContext(ctx, "release worker stopped gracefully")
			return
		case <-ticker.C:
			// Process release
			if err := w.svc.ReleaseOldMaintenances(ctx); err != nil {
				w.logger.ErrorContext(ctx, "release cycle encountered an error", slog.String("error", err.Error()))
			}
		}
	}
}
