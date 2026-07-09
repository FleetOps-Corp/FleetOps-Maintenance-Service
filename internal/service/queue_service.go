package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/fleetops/maintenance/internal/domain"
	"github.com/fleetops/maintenance/internal/port"
)

// QueueService handles queries against the maintenance queue.
//
// SAD Reference: Process Network 3 — "Consulta de Cola de Mantenimientos"
// Pattern: Service Layer (PoEAA)
type QueueService struct {
	repo           port.MaintenanceRepository
	eventPublisher port.EventPublisher
	logger         *slog.Logger
}

// NewQueueService constructs a QueueService with its dependencies injected.
//
// Pattern: Dependency Injection (ADR-7)
func NewQueueService(repo port.MaintenanceRepository, eventPublisher port.EventPublisher, logger *slog.Logger) *QueueService {
	return &QueueService{
		repo:           repo,
		eventPublisher: eventPublisher,
		logger:         logger,
	}
}

// ListQueued retrieves all maintenance records currently in the queue.
//
// SAD Reference: Process Network 3 — "en cola"
func (s *QueueService) ListQueued(ctx context.Context) ([]*domain.Maintenance, error) {
	items, err := s.repo.ListByStatus(ctx, domain.MaintenanceStatusQueued)
	if err != nil {
		s.logger.ErrorContext(
			ctx, "failed to list queued maintenances",
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("listing queued maintenances: %w", err)
	}
	return items, nil
}

// ListInProgress retrieves all maintenance records currently being processed.
//
// SAD Reference: Process Network 3 — "en mantenimiento"
func (s *QueueService) ListInProgress(ctx context.Context) ([]*domain.Maintenance, error) {
	items, err := s.repo.ListByStatus(ctx, domain.MaintenanceStatusInProgress)
	if err != nil {
		s.logger.ErrorContext(
			ctx, "failed to list in-progress maintenances",
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("listing in-progress maintenances: %w", err)
	}
	return items, nil
}

// ListAll retrieves all maintenance records regardless of status.
func (s *QueueService) ListAll(ctx context.Context) ([]*domain.Maintenance, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		s.logger.ErrorContext(
			ctx, "failed to list all maintenances",
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("listing all maintenances: %w", err)
	}
	return items, nil
}

// GetByID retrieves a single maintenance record by ID.
func (s *QueueService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Maintenance, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.WarnContext(
			ctx, "failed to get maintenance by ID",
			slog.String("maintenance_id", id.String()),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("getting maintenance %s: %w", id, err)
	}
	return m, nil
}

// FinalizeMaintenance marks a maintenance as completed in the database
// and publishes a COMPLETED event to SQS for the Vehicles microservice.
func (s *QueueService) FinalizeMaintenance(ctx context.Context, id uuid.UUID) error {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to fetch maintenance for finalization", slog.String("error", err.Error()))
		return fmt.Errorf("getting maintenance: %w", err)
	}

	if err := m.MarkCompleted(); err != nil {
		s.logger.ErrorContext(ctx, "failed to mark maintenance completed", slog.String("error", err.Error()))
		return fmt.Errorf("marking completed: %w", err)
	}

	if err := s.repo.UpdateStatus(ctx, m); err != nil {
		s.logger.ErrorContext(ctx, "failed to update maintenance status in db", slog.String("error", err.Error()))
		return fmt.Errorf("updating status in db: %w", err)
	}

	if err := s.eventPublisher.PublishMaintenanceEvent(ctx, m, "COMPLETED"); err != nil {
		s.logger.ErrorContext(ctx, "failed to publish COMPLETED event to SQS", slog.String("error", err.Error()))
		// Retornamos el error aunque la bd se haya actualizado
		return fmt.Errorf("publishing event: %w", err)
	}

	s.logger.InfoContext(ctx, "maintenance finalized and event published", slog.String("maintenance_id", m.ID.String()))
	return nil
}
