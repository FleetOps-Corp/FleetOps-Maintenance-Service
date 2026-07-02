package port

import (
	"context"

	"github.com/fleetops/maintenance/internal/domain"
)

// VehicleClient defines the contract for communicating with the external
// Vehicles microservice. This interface acts as an Anti-Corruption Layer (ACL)
// port, ensuring the maintenance domain is not contaminated by external models.
//
// [Archetype Convention Addition] — Anti-Corruption Layer (DDD best practice)
// Pattern: Anti-Corruption Layer (DDD), Dependency Inversion Principle (SOLID)
// SAD Reference: Process Network 2 — "GET /vehiculos" and "PUT /vehiculos"
type VehicleClient interface {
	// GetAllVehicles retrieves all vehicles from the external Vehicles microservice
	// and translates them into domain.Vehicle value objects.
	// SAD Reference: Process Network 2 — Step 2-3
	GetAllVehicles(ctx context.Context) ([]*domain.Vehicle, error)

	// UpdateVehicleMaintenanceStatus notifies the Vehicles microservice that a
	// vehicle's maintenance has been completed, resetting the days counter.
	UpdateVehicleMaintenanceStatus(ctx context.Context, vehicleID string) error
}
