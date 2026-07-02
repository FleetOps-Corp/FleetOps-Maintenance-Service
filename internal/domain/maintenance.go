package domain

import (
	"time"

	"github.com/google/uuid"
)

// MaintenanceType represents the type of maintenance operation.
// SAD Reference: corrective and preventive maintenance types.
type MaintenanceType string

const (
	// MaintenanceTypeCorrective represents a corrective maintenance triggered by an incident.
	MaintenanceTypeCorrective MaintenanceType = "corrective"

	// MaintenanceTypePreventive represents a preventive maintenance triggered by periodic scheduling.
	MaintenanceTypePreventive MaintenanceType = "preventive"
)

// ValidMaintenanceType checks if the given type is a recognized maintenance type.
func ValidMaintenanceType(t MaintenanceType) bool {
	return t == MaintenanceTypeCorrective || t == MaintenanceTypePreventive
}

// MaintenanceStatus represents the lifecycle state of a maintenance record.
// SAD Reference: "cola de mantenimiento" (queued), "en mantenimiento" (in_progress).
type MaintenanceStatus string

const (
	// MaintenanceStatusQueued indicates the maintenance is waiting in the queue.
	MaintenanceStatusQueued MaintenanceStatus = "queued"

	// MaintenanceStatusInProgress indicates the maintenance is currently being processed by a worker.
	MaintenanceStatusInProgress MaintenanceStatus = "in_progress"

	// MaintenanceStatusCompleted indicates the maintenance has been successfully completed.
	MaintenanceStatusCompleted MaintenanceStatus = "completed"

	// MaintenanceStatusFailed indicates the maintenance processing failed.
	MaintenanceStatusFailed MaintenanceStatus = "failed"
)

// ValidMaintenanceStatus checks if the given status is a recognized maintenance status.
func ValidMaintenanceStatus(s MaintenanceStatus) bool {
	switch s {
	case MaintenanceStatusQueued, MaintenanceStatusInProgress,
		MaintenanceStatusCompleted, MaintenanceStatusFailed:
		return true
	}
	return false
}

// Maintenance is the core domain entity representing a vehicle maintenance record.
// It encapsulates both corrective (incident-triggered) and preventive (cron-triggered)
// maintenance operations.
//
// SAD Reference: "Servicio de Mantenimiento Correctivo", "Servicio de Mantenimiento Preventivo"
// Pattern: Domain Model (PoEAA)
//
// State Machine:
//
//	queued → in_progress → completed
//	                    → failed
type Maintenance struct {
	ID          uuid.UUID
	VehicleID   string
	IncidentID  *string // nil for preventive maintenance
	Type        MaintenanceType
	Severity    uint8 // 1-10 for corrective, 0 for preventive
	Status      MaintenanceStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time // nil until completed
}

// NewCorrectiveMaintenance creates a new corrective maintenance record.
// It validates all inputs according to domain rules before constructing the entity.
//
// SAD Reference: Process Network 1 — Step 5
// Parameters: id_incidente (UUID), id_vehiculo (UUID), gravedad (uint8)
func NewCorrectiveMaintenance(vehicleID, incidentID string, severity uint8) (*Maintenance, error) {
	if vehicleID == "" {
		return nil, ErrInvalidVehicleID
	}
	if incidentID == "" {
		return nil, ErrInvalidIncidentID
	}
	if severity < 1 || severity > 10 {
		return nil, ErrInvalidSeverity
	}

	now := time.Now().UTC()
	return &Maintenance{
		ID:         uuid.New(),
		VehicleID:  vehicleID,
		IncidentID: &incidentID,
		Type:       MaintenanceTypeCorrective,
		Severity:   severity,
		Status:     MaintenanceStatusQueued,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// NewPreventiveMaintenance creates a new preventive maintenance record.
// Preventive maintenances have no incident association and zero severity.
//
// SAD Reference: Process Network 2 — Step 5
func NewPreventiveMaintenance(vehicleID string) (*Maintenance, error) {
	if vehicleID == "" {
		return nil, ErrInvalidVehicleID
	}

	now := time.Now().UTC()
	return &Maintenance{
		ID:        uuid.New(),
		VehicleID: vehicleID,
		Type:      MaintenanceTypePreventive,
		Severity:  0,
		Status:    MaintenanceStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// MarkInProgress transitions the maintenance from queued to in_progress.
// Only valid when current status is queued.
func (m *Maintenance) MarkInProgress() error {
	if m.Status != MaintenanceStatusQueued {
		return ErrInvalidStatusTransition
	}
	m.Status = MaintenanceStatusInProgress
	m.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkCompleted transitions the maintenance from in_progress to completed.
// Only valid when current status is in_progress.
func (m *Maintenance) MarkCompleted() error {
	if m.Status != MaintenanceStatusInProgress {
		return ErrInvalidStatusTransition
	}
	now := time.Now().UTC()
	m.Status = MaintenanceStatusCompleted
	m.CompletedAt = &now
	m.UpdatedAt = now
	return nil
}

// MarkFailed transitions the maintenance from in_progress to failed.
// Only valid when current status is in_progress.
func (m *Maintenance) MarkFailed() error {
	if m.Status != MaintenanceStatusInProgress {
		return ErrInvalidStatusTransition
	}
	m.Status = MaintenanceStatusFailed
	m.UpdatedAt = time.Now().UTC()
	return nil
}

// IsQueued returns true if the maintenance is currently in the queue.
func (m *Maintenance) IsQueued() bool {
	return m.Status == MaintenanceStatusQueued
}

// IsInProgress returns true if the maintenance is currently being processed.
func (m *Maintenance) IsInProgress() bool {
	return m.Status == MaintenanceStatusInProgress
}

// IsTerminal returns true if the maintenance is in a final state (completed or failed).
func (m *Maintenance) IsTerminal() bool {
	return m.Status == MaintenanceStatusCompleted || m.Status == MaintenanceStatusFailed
}
