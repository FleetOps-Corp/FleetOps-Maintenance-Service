package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/fleetops/maintenance/internal/domain"
)

// MaintenanceRepository defines the persistence contract for maintenance records.
// This interface belongs to the Business Logic Layer (Port) and must be implemented
// by the Data Access Layer (Adapter).
//
// Pattern: Repository (PoEAA) — Hexagonal Port
// Pattern: Dependency Inversion Principle (SOLID)
// SAD Reference: ADR-4 — "Repository para desacoplar la lógica de negocio
// de la capa de acceso a datos"
type MaintenanceRepository interface {
	// Create persists a new maintenance record to the database.
	Create(ctx context.Context, maintenance *domain.Maintenance) error

	// GetByID retrieves a single maintenance record by its unique identifier.
	// Returns domain.ErrMaintenanceNotFound if no record exists.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Maintenance, error)

	// List retrieves all maintenance records.
	List(ctx context.Context) ([]*domain.Maintenance, error)

	// ListByStatus retrieves maintenance records filtered by their current status.
	// SAD Reference: Process Network 3 — "listado de vehículos: en cola, en mantenimiento"
	ListByStatus(ctx context.Context, status domain.MaintenanceStatus) ([]*domain.Maintenance, error)

	// UpdateStatus persists status changes (and related fields like CompletedAt)
	// for an existing maintenance record.
	UpdateStatus(ctx context.Context, maintenance *domain.Maintenance) error

	// ListOldUncompleted retrieves maintenance records that are not in a terminal state
	// and were created more than the specified number of minutes ago.
	ListOldUncompleted(ctx context.Context, minutes int) ([]*domain.Maintenance, error)
}
