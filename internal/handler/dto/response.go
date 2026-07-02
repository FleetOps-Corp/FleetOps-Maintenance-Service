package dto

import (
	"time"
	"github.com/google/uuid"
	"github.com/fleetops/maintenance/internal/domain"
)

// MaintenanceResponse represents a single maintenance record in API responses.
//
// Pattern: Data Transfer Object (DTO)
type MaintenanceResponse struct {
	ID          uuid.UUID  `json:"id"`
	VehicleID   string     `json:"id_vehiculo"`
	IncidentID  *string    `json:"id_incidente,omitempty"`
	Type        string     `json:"tipo"`
	Severity    uint8      `json:"gravedad"`
	Status      string     `json:"estado"`
	CreatedAt   time.Time  `json:"creado_en"`
	UpdatedAt   time.Time  `json:"actualizado_en"`
	CompletedAt *time.Time `json:"completado_en,omitempty"`
}

// FromDomain converts a domain.Maintenance entity to a MaintenanceResponse DTO.
func FromDomain(m *domain.Maintenance) *MaintenanceResponse {
	return &MaintenanceResponse{
		ID:          m.ID,
		VehicleID:   m.VehicleID,
		IncidentID:  m.IncidentID,
		Type:        string(m.Type),
		Severity:    m.Severity,
		Status:      string(m.Status),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
		CompletedAt: m.CompletedAt,
	}
}

// FromDomainList converts a slice of domain.Maintenance entities to DTOs.
func FromDomainList(items []*domain.Maintenance) []*MaintenanceResponse {
	result := make([]*MaintenanceResponse, len(items))
	for i, m := range items {
		result[i] = FromDomain(m)
	}
	return result
}

// ErrorResponse represents a structured error response.
//
// [Archetype Convention Addition] — Standard HTTP error response format
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// QueueSummaryResponse represents the maintenance queue summary returned
// by Process Network 3.
//
// SAD Reference: Process Network 3 — "listado de vehículos: en cola, en mantenimiento"
type QueueSummaryResponse struct {
	Queued      []*MaintenanceResponse `json:"en_cola"`
	InProgress  []*MaintenanceResponse `json:"en_mantenimiento"`
	TotalQueued int                    `json:"total_en_cola"`
	TotalActive int                    `json:"total_en_mantenimiento"`
}

// ReportResponse represents a simplified view of a maintenance record for reports.
type ReportResponse struct {
	VehicleID            string `json:"id_vehiculo"`
	MaintenanceDate      string `json:"fecha_de_mantenimiento"`
	MaintenanceStatus    bool   `json:"estado_mantenimiento"`
	Severity             string `json:"gravedad"`
}
