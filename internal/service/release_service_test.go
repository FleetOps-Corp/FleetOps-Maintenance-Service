package service_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/fleetops/maintenance/internal/domain"
	"github.com/fleetops/maintenance/internal/mocks"
	"github.com/fleetops/maintenance/internal/service"
)

func TestReleaseService_ReleaseOldMaintenances_Success(t *testing.T) {
	repo := new(mocks.MockMaintenanceRepository)
	publisher := new(mocks.MockEventPublisher)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	svc := service.NewReleaseService(repo, publisher, logger, 30)

	id := uuid.New()
	m := &domain.Maintenance{
		ID:        id,
		Status:    domain.MaintenanceStatusInProgress,
		VehicleID: "ABC-123",
	}

	repo.On("ListOldUncompleted", mock.Anything, 30).Return([]*domain.Maintenance{m}, nil)
	publisher.On("PublishMaintenanceEvent", mock.Anything, m, "COMPLETED").Return(nil)
	repo.On("UpdateStatus", mock.Anything, m).Return(nil)

	err := svc.ReleaseOldMaintenances(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, domain.MaintenanceStatusCompleted, m.Status)
	repo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestReleaseService_ReleaseOldMaintenances_SqsError(t *testing.T) {
	repo := new(mocks.MockMaintenanceRepository)
	publisher := new(mocks.MockEventPublisher)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	svc := service.NewReleaseService(repo, publisher, logger, 30)

	m := &domain.Maintenance{
		ID:        uuid.New(),
		Status:    domain.MaintenanceStatusInProgress,
		VehicleID: "ABC-123",
	}

	repo.On("ListOldUncompleted", mock.Anything, 30).Return([]*domain.Maintenance{m}, nil)
	publisher.On("PublishMaintenanceEvent", mock.Anything, m, "COMPLETED").Return(errors.New("sqs error"))
	// UpdateStatus should NOT be called

	err := svc.ReleaseOldMaintenances(context.Background())
	assert.NoError(t, err) // It continues on error
	repo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}
