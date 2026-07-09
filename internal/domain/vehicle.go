package domain

import "time"

// Vehicle represents the Anti-Corruption Layer (ACL) boundary model for vehicle
// data received from the external Vehicles microservice. This value object
// translates the external service's model into the maintenance domain's language,
// preventing external model contamination.
//
// [Archetype Convention Addition] — Anti-Corruption Layer (DDD best practice)
// SAD Reference: Process Network 2 — "El microservicio de vehículos retorna
// una lista con todos los vehículos existentes"
type Vehicle struct {
	ID                       string
	LicensePlate             string
	VehicleType              string
	CreatedAt                time.Time
	KilometersRecorded       float64
	DaysSinceLastMaintenance int
	Available                bool
}

// NeedsPreventiveMaintenance determines if a vehicle qualifies for preventive
// maintenance based on configurable thresholds per vehicle type.
//
// SAD Reference: Process Network 2 — Step 4: "filtra en base a parámetros
// como kilómetros recorridos y días desde el último mantenimiento"
//
// A vehicle qualifies if it is available AND either:
//   - Its recorded kilometers >= kmThreshold, OR
//   - Its days since last maintenance >= daysThreshold
func (v *Vehicle) NeedsPreventiveMaintenance(kmThresholds map[string]float64, daysThreshold int) bool {
	if !v.Available {
		return false
	}

	// Extract specific threshold or fallback to default
	kmThreshold, exists := kmThresholds[v.VehicleType]
	if !exists || v.VehicleType == "" {
		kmThreshold = 10000.0 // Default safety threshold
	}

	return v.KilometersRecorded >= kmThreshold || v.DaysSinceLastMaintenance >= daysThreshold
}
