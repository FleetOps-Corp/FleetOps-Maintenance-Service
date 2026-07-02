package dto

// CreateMaintenanceRequest represents the JSON payload received when creating
// a new corrective maintenance record.
//
// Pattern: Data Transfer Object (DTO)
// SAD Reference: Process Network 1 — Parameters:
//
//	id_incidente : UUID
//	id_vehiculo  : UUID
//	gravedad     : uint8
type CreateMaintenanceRequest struct {
	IncidentID string `json:"id_incidente"`
	VehicleID  string `json:"id_vehiculo"`
	Severity   uint8  `json:"gravedad"`
}

// Validate checks that the request contains valid data before
// passing it to the service layer.
func (r *CreateMaintenanceRequest) Validate() error {
	if r.VehicleID == "" {
		return ErrValidation("id_vehiculo is required")
	}
	if r.IncidentID == "" {
		return ErrValidation("id_incidente is required")
	}
	if r.Severity < 1 || r.Severity > 10 {
		return ErrValidation("gravedad must be between 1 and 10")
	}
	return nil
}

// ErrValidation represents a validation error in a request DTO.
type ErrValidation string

func (e ErrValidation) Error() string {
	return string(e)
}
