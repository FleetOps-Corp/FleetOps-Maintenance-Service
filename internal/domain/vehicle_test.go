package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fleetops/maintenance/internal/domain"
)

func TestNeedsPreventiveMaintenance_KmThresholdExceeded(t *testing.T) {
	// Arrange
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		KilometersRecorded:       15000,
		DaysSinceLastMaintenance: 30,
		Available:                true,
	}

	// Act & Assert
	assert.True(t, v.NeedsPreventiveMaintenance(10000, 90))
}

func TestNeedsPreventiveMaintenance_DaysThresholdExceeded(t *testing.T) {
	// Arrange
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		KilometersRecorded:       5000,
		DaysSinceLastMaintenance: 100,
		Available:                true,
	}

	// Act & Assert
	assert.True(t, v.NeedsPreventiveMaintenance(10000, 90))
}

func TestNeedsPreventiveMaintenance_BothThresholdsExceeded(t *testing.T) {
	// Arrange
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		KilometersRecorded:       20000,
		DaysSinceLastMaintenance: 120,
		Available:                true,
	}

	// Act & Assert
	assert.True(t, v.NeedsPreventiveMaintenance(10000, 90))
}

func TestNeedsPreventiveMaintenance_NeitherThresholdExceeded(t *testing.T) {
	// Arrange
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		KilometersRecorded:       5000,
		DaysSinceLastMaintenance: 30,
		Available:                true,
	}

	// Act & Assert
	assert.False(t, v.NeedsPreventiveMaintenance(10000, 90))
}

func TestNeedsPreventiveMaintenance_NotAvailable(t *testing.T) {
	// Arrange
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		KilometersRecorded:       20000,
		DaysSinceLastMaintenance: 120,
		Available:                false,
	}

	// Act & Assert — unavailable vehicles never qualify
	assert.False(t, v.NeedsPreventiveMaintenance(10000, 90))
}

func TestNeedsPreventiveMaintenance_ExactKmThreshold(t *testing.T) {
	// Arrange
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		KilometersRecorded:       10000,
		DaysSinceLastMaintenance: 30,
		Available:                true,
	}

	// Act & Assert — exact threshold should qualify (>=)
	assert.True(t, v.NeedsPreventiveMaintenance(10000, 90))
}

func TestNeedsPreventiveMaintenance_ExactDaysThreshold(t *testing.T) {
	// Arrange
	v := &domain.Vehicle{
		ID:                       "ABC-123",
		KilometersRecorded:       5000,
		DaysSinceLastMaintenance: 90,
		Available:                true,
	}

	// Act & Assert — exact threshold should qualify (>=)
	assert.True(t, v.NeedsPreventiveMaintenance(10000, 90))
}
