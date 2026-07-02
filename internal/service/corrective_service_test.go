package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fleetops/maintenance/internal/domain"
	"github.com/fleetops/maintenance/internal/mocks"
	"github.com/fleetops/maintenance/internal/service"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// =============================================================================
// CreateCorrective tests
// =============================================================================

func TestCreateCorrective_Success(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	svc := service.NewCorrectiveMaintenanceService(repo, newTestLogger())

	vehicleID := "ABC-123"
	incidentID := "INC-123"
	severity := uint8(5)

	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Maintenance")).Return(nil)

	// Act
	m, err := svc.CreateCorrective(context.Background(), vehicleID, incidentID, severity)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.Equal(t, vehicleID, m.VehicleID)
	assert.Equal(t, &incidentID, m.IncidentID)
	assert.Equal(t, domain.MaintenanceTypeCorrective, m.Type)
	assert.Equal(t, severity, m.Severity)
	assert.Equal(t, domain.MaintenanceStatusQueued, m.Status)
	repo.AssertExpectations(t)
}

func TestCreateCorrective_InvalidVehicleID(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	svc := service.NewCorrectiveMaintenanceService(repo, newTestLogger())

	// Act
	m, err := svc.CreateCorrective(context.Background(), "", "INC-123", 5)

	// Assert
	assert.Nil(t, m)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidVehicleID)
	repo.AssertNotCalled(t, "Create")
}

func TestCreateCorrective_InvalidIncidentID(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	svc := service.NewCorrectiveMaintenanceService(repo, newTestLogger())

	// Act
	m, err := svc.CreateCorrective(context.Background(), "ABC-123", "", 5)

	// Assert
	assert.Nil(t, m)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidIncidentID)
	repo.AssertNotCalled(t, "Create")
}

func TestCreateCorrective_InvalidSeverity(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	svc := service.NewCorrectiveMaintenanceService(repo, newTestLogger())

	// Act
	m, err := svc.CreateCorrective(context.Background(), "ABC-123", "INC-123", 0)

	// Assert
	assert.Nil(t, m)
	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidSeverity)
	repo.AssertNotCalled(t, "Create")
}

func TestCreateCorrective_RepositoryError(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	svc := service.NewCorrectiveMaintenanceService(repo, newTestLogger())

	dbErr := errors.New("database connection lost")
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Maintenance")).Return(dbErr)

	// Act
	m, err := svc.CreateCorrective(context.Background(), "ABC-123", "INC-123", 5)

	// Assert
	assert.Nil(t, m)
	assert.Error(t, err)
	assert.ErrorIs(t, err, dbErr)
	repo.AssertExpectations(t)
}
