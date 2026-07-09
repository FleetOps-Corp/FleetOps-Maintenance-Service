package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fleetops/maintenance/internal/port"
)

// ReleaseService automates the finalization of old maintenances.
//
// SAD Reference: Process Network 3 - Automated vehicle release
// Pattern: Service Layer (PoEAA)
type ReleaseService struct {
	repo           port.MaintenanceRepository
	eventPublisher port.EventPublisher
	logger         *slog.Logger
	thresholdMins  int
}

// NewReleaseService constructs a ReleaseService with its dependencies.
func NewReleaseService(repo port.MaintenanceRepository, eventPublisher port.EventPublisher, logger *slog.Logger, thresholdMins int) *ReleaseService {
	return &ReleaseService{
		repo:           repo,
		eventPublisher: eventPublisher,
		logger:         logger,
		thresholdMins:  thresholdMins,
	}
}

// ReleaseOldMaintenances finds maintenances that have been in progress or queued
// for longer than the threshold, marks them as completed locally, and publishes
// the COMPLETED event to SQS so the vehicles microservice marks them as AVAILABLE.
func (s *ReleaseService) ReleaseOldMaintenances(ctx context.Context) error {
	// 1. Find all non-terminal maintenances older than threshold
	oldMaintenances, err := s.repo.ListOldUncompleted(ctx, s.thresholdMins)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list old uncompleted maintenances", slog.String("error", err.Error()))
		return fmt.Errorf("listing old uncompleted: %w", err)
	}

	if len(oldMaintenances) == 0 {
		return nil // Nothing to do
	}

	s.logger.InfoContext(ctx, "found old maintenances to automatically release", slog.Int("count", len(oldMaintenances)))

	// 2. Process each maintenance
	for _, m := range oldMaintenances {
		// Mark it as completed locally
		if err := m.MarkCompleted(); err != nil {
			s.logger.WarnContext(ctx, "failed to mark maintenance completed in memory", slog.String("id", m.ID.String()), slog.String("error", err.Error()))
			continue
		}

		// Publish event to SQS FIRST
		// If this fails, we skip DB update so it can be retried on next poll
		if err := s.eventPublisher.PublishMaintenanceEvent(ctx, m, "COMPLETED"); err != nil {
			s.logger.ErrorContext(ctx, "failed to publish COMPLETED event to SQS during auto-release", slog.String("id", m.ID.String()), slog.String("error", err.Error()))
			continue
		}

		// Save status to DB
		if err := s.repo.UpdateStatus(ctx, m); err != nil {
			s.logger.WarnContext(ctx, "failed to update maintenance status in db", slog.String("id", m.ID.String()), slog.String("error", err.Error()))
			continue
		}

		s.logger.InfoContext(ctx, "automatically released vehicle", slog.String("maintenance_id", m.ID.String()), slog.String("vehicle_id", m.VehicleID))
	}

	return nil
}
