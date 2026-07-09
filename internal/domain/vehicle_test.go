package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fleetops/maintenance/internal/domain"
)

var defaultThresholds = map[string]float64{
	"Automovil":              10000.0,
	"Camioneta":              12000.0,
	"Vehiculo_especializado": 15000.0,
}

func TestNeedsPreventiveMaintenance_KmThresholdExceeded(t *testing.T) {
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		VehicleType:              "Automovil",
		KilometersRecorded:       15000,
		DaysSinceLastMaintenance: 30,
		Available:                true,
	}
	assert.True(t, v.NeedsPreventiveMaintenance(defaultThresholds, 90))
}

func TestNeedsPreventiveMaintenance_DaysThresholdExceeded(t *testing.T) {
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		VehicleType:              "Camioneta",
		KilometersRecorded:       5000,
		DaysSinceLastMaintenance: 100,
		Available:                true,
	}
	assert.True(t, v.NeedsPreventiveMaintenance(defaultThresholds, 90))
}

func TestNeedsPreventiveMaintenance_BothThresholdsExceeded(t *testing.T) {
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		VehicleType:              "Vehiculo_especializado",
		KilometersRecorded:       20000,
		DaysSinceLastMaintenance: 120,
		Available:                true,
	}
	assert.True(t, v.NeedsPreventiveMaintenance(defaultThresholds, 90))
}

func TestNeedsPreventiveMaintenance_NeitherThresholdExceeded(t *testing.T) {
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		VehicleType:              "Automovil",
		KilometersRecorded:       5000,
		DaysSinceLastMaintenance: 30,
		Available:                true,
	}
	assert.False(t, v.NeedsPreventiveMaintenance(defaultThresholds, 90))
}

func TestNeedsPreventiveMaintenance_NotAvailable(t *testing.T) {
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		VehicleType:              "Automovil",
		KilometersRecorded:       20000,
		DaysSinceLastMaintenance: 120,
		Available:                false,
	}
	assert.False(t, v.NeedsPreventiveMaintenance(defaultThresholds, 90))
}

func TestNeedsPreventiveMaintenance_ExactKmThreshold(t *testing.T) {
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		VehicleType:              "Camioneta",
		KilometersRecorded:       12000,
		DaysSinceLastMaintenance: 30,
		Available:                true,
	}
	assert.True(t, v.NeedsPreventiveMaintenance(defaultThresholds, 90))
}

func TestNeedsPreventiveMaintenance_UnknownTypeDefaultsTo10000(t *testing.T) {
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		VehicleType:              "Desconocido",
		KilometersRecorded:       10500, // Exceeds default 10000, but under others
		DaysSinceLastMaintenance: 30,
		Available:                true,
	}
	assert.True(t, v.NeedsPreventiveMaintenance(defaultThresholds, 90))
}

func TestNeedsPreventiveMaintenance_EmptyTypeDefaultsTo10000(t *testing.T) {
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		VehicleType:              "",
		KilometersRecorded:       9000, // Under default 10000
		DaysSinceLastMaintenance: 30,
		Available:                true,
	}
	assert.False(t, v.NeedsPreventiveMaintenance(defaultThresholds, 90))
}
