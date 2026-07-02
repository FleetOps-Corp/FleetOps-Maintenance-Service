package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fleetops/maintenance/internal/domain"
)

// =============================================================================
// NewCorrectiveMaintenance tests
// =============================================================================

func TestNewCorrectiveMaintenance_Success(t *testing.T) {
	// Arrange
	vehicleID := "ABC-123"
	incidentID := "INC-123"
	severity := uint8(5)

	// Act
	m, err := domain.NewCorrectiveMaintenance(vehicleID, incidentID, severity)

	// Assert
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, m.ID)
	assert.Equal(t, vehicleID, m.VehicleID)
	assert.Equal(t, &incidentID, m.IncidentID)
	assert.Equal(t, domain.MaintenanceTypeCorrective, m.Type)
	assert.Equal(t, severity, m.Severity)
	assert.Equal(t, domain.MaintenanceStatusQueued, m.Status)
	assert.NotZero(t, m.CreatedAt)
	assert.NotZero(t, m.UpdatedAt)
	assert.Nil(t, m.CompletedAt)
}

func TestNewCorrectiveMaintenance_InvalidVehicleID(t *testing.T) {
	// Arrange & Act
	m, err := domain.NewCorrectiveMaintenance("", "INC-123", 5)

	// Assert
	assert.Nil(t, m)
	assert.ErrorIs(t, err, domain.ErrInvalidVehicleID)
}

func TestNewCorrectiveMaintenance_InvalidIncidentID(t *testing.T) {
	// Arrange & Act
	m, err := domain.NewCorrectiveMaintenance("ABC-123", "", 5)

	// Assert
	assert.Nil(t, m)
	assert.ErrorIs(t, err, domain.ErrInvalidIncidentID)
}

func TestNewCorrectiveMaintenance_SeverityTooLow(t *testing.T) {
	// Arrange & Act
	m, err := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 0)

	// Assert
	assert.Nil(t, m)
	assert.ErrorIs(t, err, domain.ErrInvalidSeverity)
}

func TestNewCorrectiveMaintenance_SeverityTooHigh(t *testing.T) {
	// Arrange & Act
	m, err := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 11)

	// Assert
	assert.Nil(t, m)
	assert.ErrorIs(t, err, domain.ErrInvalidSeverity)
}

func TestNewCorrectiveMaintenance_SeverityBoundary_Min(t *testing.T) {
	// Arrange & Act
	m, err := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 1)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, uint8(1), m.Severity)
}

func TestNewCorrectiveMaintenance_SeverityBoundary_Max(t *testing.T) {
	// Arrange & Act
	m, err := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 10)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, uint8(10), m.Severity)
}

// =============================================================================
// NewPreventiveMaintenance tests
// =============================================================================

func TestNewPreventiveMaintenance_Success(t *testing.T) {
	// Arrange
	vehicleID := "ABC-123"

	// Act
	m, err := domain.NewPreventiveMaintenance(vehicleID)

	// Assert
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, m.ID)
	assert.Equal(t, vehicleID, m.VehicleID)
	assert.Nil(t, m.IncidentID)
	assert.Equal(t, domain.MaintenanceTypePreventive, m.Type)
	assert.Equal(t, uint8(0), m.Severity)
	assert.Equal(t, domain.MaintenanceStatusQueued, m.Status)
}

func TestNewPreventiveMaintenance_InvalidVehicleID(t *testing.T) {
	// Arrange & Act
	m, err := domain.NewPreventiveMaintenance("")

	// Assert
	assert.Nil(t, m)
	assert.ErrorIs(t, err, domain.ErrInvalidVehicleID)
}

// =============================================================================
// State machine transition tests
// =============================================================================

func TestMarkInProgress_FromQueued_Success(t *testing.T) {
	// Arrange
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)

	// Act
	err := m.MarkInProgress()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, domain.MaintenanceStatusInProgress, m.Status)
}

func TestMarkInProgress_FromInProgress_Fails(t *testing.T) {
	// Arrange
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)
	_ = m.MarkInProgress()

	// Act
	err := m.MarkInProgress()

	// Assert
	assert.ErrorIs(t, err, domain.ErrInvalidStatusTransition)
}

func TestMarkInProgress_FromCompleted_Fails(t *testing.T) {
	// Arrange
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)
	_ = m.MarkInProgress()
	_ = m.MarkCompleted()

	// Act
	err := m.MarkInProgress()

	// Assert
	assert.ErrorIs(t, err, domain.ErrInvalidStatusTransition)
}

func TestMarkCompleted_FromInProgress_Success(t *testing.T) {
	// Arrange
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)
	_ = m.MarkInProgress()

	// Act
	err := m.MarkCompleted()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, domain.MaintenanceStatusCompleted, m.Status)
	assert.NotNil(t, m.CompletedAt)
}

func TestMarkCompleted_FromQueued_Fails(t *testing.T) {
	// Arrange
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)

	// Act
	err := m.MarkCompleted()

	// Assert
	assert.ErrorIs(t, err, domain.ErrInvalidStatusTransition)
}

func TestMarkFailed_FromInProgress_Success(t *testing.T) {
	// Arrange
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)
	_ = m.MarkInProgress()

	// Act
	err := m.MarkFailed()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, domain.MaintenanceStatusFailed, m.Status)
}

func TestMarkFailed_FromQueued_Fails(t *testing.T) {
	// Arrange
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)

	// Act
	err := m.MarkFailed()

	// Assert
	assert.ErrorIs(t, err, domain.ErrInvalidStatusTransition)
}

// =============================================================================
// Status query method tests
// =============================================================================

func TestIsQueued_WhenQueued(t *testing.T) {
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)
	assert.True(t, m.IsQueued())
}

func TestIsQueued_WhenNotQueued(t *testing.T) {
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)
	_ = m.MarkInProgress()
	assert.False(t, m.IsQueued())
}

func TestIsInProgress_WhenInProgress(t *testing.T) {
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)
	_ = m.MarkInProgress()
	assert.True(t, m.IsInProgress())
}

func TestIsInProgress_WhenNotInProgress(t *testing.T) {
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)
	assert.False(t, m.IsInProgress())
}

func TestIsTerminal_WhenCompleted(t *testing.T) {
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)
	_ = m.MarkInProgress()
	_ = m.MarkCompleted()
	assert.True(t, m.IsTerminal())
}

func TestIsTerminal_WhenFailed(t *testing.T) {
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)
	_ = m.MarkInProgress()
	_ = m.MarkFailed()
	assert.True(t, m.IsTerminal())
}

func TestIsTerminal_WhenQueued(t *testing.T) {
	m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)
	assert.False(t, m.IsTerminal())
}

// =============================================================================
// Type and Status validation tests
// =============================================================================

func TestValidMaintenanceType(t *testing.T) {
	assert.True(t, domain.ValidMaintenanceType(domain.MaintenanceTypeCorrective))
	assert.True(t, domain.ValidMaintenanceType(domain.MaintenanceTypePreventive))
	assert.False(t, domain.ValidMaintenanceType("unknown"))
}

func TestValidMaintenanceStatus(t *testing.T) {
	assert.True(t, domain.ValidMaintenanceStatus(domain.MaintenanceStatusQueued))
	assert.True(t, domain.ValidMaintenanceStatus(domain.MaintenanceStatusInProgress))
	assert.True(t, domain.ValidMaintenanceStatus(domain.MaintenanceStatusCompleted))
	assert.True(t, domain.ValidMaintenanceStatus(domain.MaintenanceStatusFailed))
	assert.False(t, domain.ValidMaintenanceStatus("unknown"))
}
