package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fleetops/maintenance/internal/domain"
	"github.com/fleetops/maintenance/internal/mocks"
	"github.com/fleetops/maintenance/internal/service"
)

// =============================================================================
// SchedulePreventive tests
// =============================================================================

func TestSchedulePreventive_Success_FiltersAndCreates(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	vehicleClient := new(mocks.MockVehicleFetcher)
	publisher := new(mocks.MockEventPublisher)
	kmThresholds := map[string]float64{"Automovil": 10000.0}
	svc := service.NewPreventiveMaintenanceService(
		repo, vehicleClient, publisher, kmThresholds, 90, 7, newTestLogger(),
	)

	vehicles := []*domain.Vehicle{
		{ID: "V1", VehicleType: "Automovil", KilometersRecorded: 15000, DaysSinceLastMaintenance: 30, Available: true},   // qualifies (km)
		{ID: "V2", VehicleType: "Automovil", KilometersRecorded: 5000, DaysSinceLastMaintenance: 100, Available: true},   // qualifies (days)
		{ID: "V3", VehicleType: "Automovil", KilometersRecorded: 3000, DaysSinceLastMaintenance: 20, Available: true},    // does NOT qualify
		{ID: "V4", VehicleType: "Automovil", KilometersRecorded: 20000, DaysSinceLastMaintenance: 120, Available: false}, // NOT available
	}

	vehicleClient.On("GetAllVehicles", mock.Anything).Return(vehicles, nil)
	repo.On("ListByStatus", mock.Anything, mock.Anything).Return([]*domain.Maintenance{}, nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Maintenance")).Return(nil)
	publisher.On("PublishMaintenanceEvent", mock.Anything, mock.AnythingOfType("*domain.Maintenance"), "CREATED").Return(nil)

	// Act
	created, err := svc.SchedulePreventive(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Len(t, created, 2) // only 2 vehicles qualify
	for _, m := range created {
		assert.Equal(t, domain.MaintenanceTypePreventive, m.Type)
		assert.Equal(t, domain.MaintenanceStatusQueued, m.Status)
	}
	vehicleClient.AssertExpectations(t)
	repo.AssertNumberOfCalls(t, "Create", 2)
}

func TestSchedulePreventive_NoVehiclesQualify(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	vehicleClient := new(mocks.MockVehicleFetcher)
	publisher := new(mocks.MockEventPublisher)
	kmThresholds := map[string]float64{"Automovil": 10000.0}
	svc := service.NewPreventiveMaintenanceService(
		repo, vehicleClient, publisher, kmThresholds, 90, 7, newTestLogger(),
	)

	vehicles := []*domain.Vehicle{
		{ID: "V1", VehicleType: "Automovil", KilometersRecorded: 5000, DaysSinceLastMaintenance: 30, Available: true},
	}

	vehicleClient.On("GetAllVehicles", mock.Anything).Return(vehicles, nil)
	repo.On("ListByStatus", mock.Anything, mock.Anything).Return([]*domain.Maintenance{}, nil)

	// Act
	created, err := svc.SchedulePreventive(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Empty(t, created)
	repo.AssertNotCalled(t, "Create")
}

func TestSchedulePreventive_VehicleClientError(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	vehicleClient := new(mocks.MockVehicleFetcher)
	publisher := new(mocks.MockEventPublisher)
	kmThresholds := map[string]float64{"Automovil": 10000.0}
	svc := service.NewPreventiveMaintenanceService(
		repo, vehicleClient, publisher, kmThresholds, 90, 7, newTestLogger(),
	)

	vehicleClient.On("GetAllVehicles", mock.Anything).Return(nil, errors.New("connection refused"))

	// Act
	created, err := svc.SchedulePreventive(context.Background())

	// Assert
	assert.Nil(t, created)
	assert.Error(t, err)
	repo.AssertNotCalled(t, "Create")
}

func TestSchedulePreventive_RepositoryError_ContinuesProcessing(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	vehicleClient := new(mocks.MockVehicleFetcher)
	publisher := new(mocks.MockEventPublisher)
	kmThresholds := map[string]float64{"Automovil": 10000.0}
	svc := service.NewPreventiveMaintenanceService(
		repo, vehicleClient, publisher, kmThresholds, 90, 7, newTestLogger(),
	)

	vehicles := []*domain.Vehicle{
		{ID: "V1", VehicleType: "Automovil", KilometersRecorded: 15000, DaysSinceLastMaintenance: 30, Available: true},
		{ID: "V2", VehicleType: "Automovil", KilometersRecorded: 20000, DaysSinceLastMaintenance: 30, Available: true},
	}

	vehicleClient.On("GetAllVehicles", mock.Anything).Return(vehicles, nil)
	repo.On("ListByStatus", mock.Anything, mock.Anything).Return([]*domain.Maintenance{}, nil)
	// First call fails, second succeeds
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Maintenance")).
		Return(errors.New("db error")).Once()
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Maintenance")).
		Return(nil).Once()
	publisher.On("PublishMaintenanceEvent", mock.Anything, mock.AnythingOfType("*domain.Maintenance"), "CREATED").Return(nil).Once()

	// Act
	created, err := svc.SchedulePreventive(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Len(t, created, 1) // only the second one succeeded
	repo.AssertNumberOfCalls(t, "Create", 2)
}

func TestSchedulePreventive_EmptyVehicleList(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	vehicleClient := new(mocks.MockVehicleFetcher)
	publisher := new(mocks.MockEventPublisher)
	kmThresholds := map[string]float64{"Automovil": 10000.0}
	svc := service.NewPreventiveMaintenanceService(
		repo, vehicleClient, publisher, kmThresholds, 90, 7, newTestLogger(),
	)

	vehicleClient.On("GetAllVehicles", mock.Anything).Return([]*domain.Vehicle{}, nil)
	repo.On("ListByStatus", mock.Anything, mock.Anything).Return([]*domain.Maintenance{}, nil)

	// Act
	created, err := svc.SchedulePreventive(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Empty(t, created)
}

func TestSchedulePreventive_SkipsActiveVehicles(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	vehicleClient := new(mocks.MockVehicleFetcher)
	publisher := new(mocks.MockEventPublisher)
	kmThresholds := map[string]float64{"Automovil": 10000.0}
	svc := service.NewPreventiveMaintenanceService(
		repo, vehicleClient, publisher, kmThresholds, 90, 7, newTestLogger(),
	)

	vehicles := []*domain.Vehicle{
		{ID: "V1", VehicleType: "Automovil", KilometersRecorded: 15000, DaysSinceLastMaintenance: 30, Available: true}, // qualifies, but is queued
		{ID: "V2", VehicleType: "Automovil", KilometersRecorded: 20000, DaysSinceLastMaintenance: 30, Available: true}, // qualifies, but is in progress
		{ID: "V3", VehicleType: "Automovil", KilometersRecorded: 25000, DaysSinceLastMaintenance: 30, Available: true}, // qualifies and is free
	}

	vehicleClient.On("GetAllVehicles", mock.Anything).Return(vehicles, nil)

	// Mock active vehicles: V1 is queued, V2 is in_progress
	mQueued, _ := domain.NewPreventiveMaintenance("V1")
	mInProgress, _ := domain.NewPreventiveMaintenance("V2")

	repo.On("ListByStatus", mock.Anything, domain.MaintenanceStatusQueued).Return([]*domain.Maintenance{mQueued}, nil)
	repo.On("ListByStatus", mock.Anything, domain.MaintenanceStatusInProgress).Return([]*domain.Maintenance{mInProgress}, nil)

	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Maintenance")).Return(nil)
	publisher.On("PublishMaintenanceEvent", mock.Anything, mock.AnythingOfType("*domain.Maintenance"), "CREATED").Return(nil)

	// Act
	created, err := svc.SchedulePreventive(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Len(t, created, 1) // Only V3 should be scheduled
	assert.Equal(t, "V3", created[0].VehicleID)
}
