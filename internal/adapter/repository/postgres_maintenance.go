package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fleetops/maintenance/internal/domain"
	"github.com/fleetops/maintenance/internal/port"
)

// Compile-time check: PostgresMaintenanceRepository implements port.MaintenanceRepository.
var _ port.MaintenanceRepository = (*PostgresMaintenanceRepository)(nil)

// PostgresMaintenanceRepository is the concrete implementation of port.MaintenanceRepository
// using pgx v5 for raw SQL queries against PostgreSQL (hosted on Supabase).
//
// Pattern: Repository (PoEAA) — Hexagonal Adapter
// SAD Reference: ADR-4, ADR-2 — "Capa de acceso a datos interactúa directamente
// con PostgreSQL a través de Supabase"
type PostgresMaintenanceRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresMaintenanceRepository constructs a PostgresMaintenanceRepository.
func NewPostgresMaintenanceRepository(pool *pgxpool.Pool) *PostgresMaintenanceRepository {
	return &PostgresMaintenanceRepository{pool: pool}
}

// Create inserts a new maintenance record into the database.
func (r *PostgresMaintenanceRepository) Create(ctx context.Context, m *domain.Maintenance) error {
	query := `
		INSERT INTO maintenances (id, vehicle_id, incident_id, type, severity, status, created_at, updated_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(
		ctx, query,
		m.ID, m.VehicleID, m.IncidentID, m.Type, m.Severity,
		m.Status, m.CreatedAt, m.UpdatedAt, m.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting maintenance: %w", err)
	}
	return nil
}

// GetByID retrieves a maintenance record by its UUID.
func (r *PostgresMaintenanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Maintenance, error) {
	query := `
		SELECT id, vehicle_id, incident_id, type, severity, status, created_at, updated_at, completed_at
		FROM maintenances
		WHERE id = $1`

	m := &domain.Maintenance{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&m.ID, &m.VehicleID, &m.IncidentID, &m.Type, &m.Severity,
		&m.Status, &m.CreatedAt, &m.UpdatedAt, &m.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMaintenanceNotFound
		}
		return nil, fmt.Errorf("querying maintenance by ID: %w", err)
	}
	return m, nil
}

// List retrieves all maintenance records ordered by creation date (newest first).
func (r *PostgresMaintenanceRepository) List(ctx context.Context) ([]*domain.Maintenance, error) {
	query := `
		SELECT id, vehicle_id, incident_id, type, severity, status, created_at, updated_at, completed_at
		FROM maintenances
		ORDER BY created_at DESC`

	return r.queryMultiple(ctx, query)
}

// ListByStatus retrieves maintenance records filtered by status.
// SAD Reference: Process Network 3 — "en cola" / "en mantenimiento"
func (r *PostgresMaintenanceRepository) ListByStatus(ctx context.Context, status domain.MaintenanceStatus) ([]*domain.Maintenance, error) {
	query := `
		SELECT id, vehicle_id, incident_id, type, severity, status, created_at, updated_at, completed_at
		FROM maintenances
		WHERE status = $1
		ORDER BY created_at ASC`

	return r.queryMultiple(ctx, query, status)
}

// UpdateStatus persists the current status (and related fields) of a maintenance record.
func (r *PostgresMaintenanceRepository) UpdateStatus(ctx context.Context, m *domain.Maintenance) error {
	query := `
		UPDATE maintenances
		SET status = $1, updated_at = $2, completed_at = $3
		WHERE id = $4`

	result, err := r.pool.Exec(ctx, query, m.Status, m.UpdatedAt, m.CompletedAt, m.ID)
	if err != nil {
		return fmt.Errorf("updating maintenance status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrMaintenanceNotFound
	}
	return nil
}

// queryMultiple is a helper that executes a query and scans multiple maintenance rows.
func (r *PostgresMaintenanceRepository) queryMultiple(ctx context.Context, query string, args ...any) ([]*domain.Maintenance, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying maintenances: %w", err)
	}
	defer rows.Close()

	var results []*domain.Maintenance
	for rows.Next() {
		m := &domain.Maintenance{}
		if err := rows.Scan(
			&m.ID, &m.VehicleID, &m.IncidentID, &m.Type, &m.Severity,
			&m.Status, &m.CreatedAt, &m.UpdatedAt, &m.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning maintenance row: %w", err)
		}
		results = append(results, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating maintenance rows: %w", err)
	}

	return results, nil
}

// ListOldUncompleted retrieves records that are not in a terminal state
// and were created more than 'minutes' ago.
func (r *PostgresMaintenanceRepository) ListOldUncompleted(ctx context.Context, minutes int) ([]*domain.Maintenance, error) {
	query := `
		SELECT id, vehicle_id, incident_id, type, severity, status, created_at, updated_at, completed_at
		FROM maintenances
		WHERE status NOT IN ('completed', 'failed')
		AND created_at < NOW() - ($1 || ' minutes')::INTERVAL
		ORDER BY created_at ASC`

	return r.queryMultiple(ctx, query, minutes)
}
