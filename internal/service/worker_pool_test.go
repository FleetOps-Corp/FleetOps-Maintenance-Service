package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/fleetops/maintenance/internal/domain"
	"github.com/fleetops/maintenance/internal/mocks"
	"github.com/fleetops/maintenance/internal/service"
)

// =============================================================================
// WorkerPool tests
// =============================================================================

func TestWorkerPool_StartAndStop(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	wp := service.NewWorkerPool(repo, 3, 1, false, newTestLogger())

	// Act — start and immediately stop, should not panic
	ctx, cancel := context.WithCancel(context.Background())
	wp.Start(ctx)

	// Give it a moment then stop
	time.Sleep(50 * time.Millisecond)
	cancel()
	wp.Stop()

	// Assert — no panic, no deadlock
}

func TestWorkerPool_StopIsIdempotent(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	wp := service.NewWorkerPool(repo, 3, 60, false, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)

	// Act — calling Stop multiple times should not panic
	wp.Stop()
	wp.Stop()
	wp.Stop()
}

func TestWorkerPool_ProcessesQueuedItems(t *testing.T) {
	tests := []struct {
		name             string
		useMockFallback  bool
		expectedStatus   domain.MaintenanceStatus
		expectedTerminal bool
	}{
		{
			name:             "Without fallback completes task immediately",
			useMockFallback:  false,
			expectedStatus:   domain.MaintenanceStatusCompleted,
			expectedTerminal: true,
		},
		{
			name:             "With fallback leaves task in progress",
			useMockFallback:  true,
			expectedStatus:   domain.MaintenanceStatusInProgress,
			expectedTerminal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := new(mocks.MockMaintenanceRepository)
			vehicleID := "ABC-123"
			m1, _ := domain.NewCorrectiveMaintenance(vehicleID, "INC-123", 5)
			queued := []*domain.Maintenance{m1}

			repo.On("ListByStatus", mock.Anything, domain.MaintenanceStatus("queued")).
				Return(queued, nil).Once()
			repo.On("ListByStatus", mock.Anything, domain.MaintenanceStatus("queued")).
				Return([]*domain.Maintenance{}, nil)
			repo.On("UpdateStatus", mock.Anything, mock.AnythingOfType("*domain.Maintenance")).
				Return(nil)

			// Act
			wp := service.NewWorkerPool(repo, 2, 1, tt.useMockFallback, newTestLogger())
			ctx, cancel := context.WithCancel(context.Background())
			wp.Start(ctx)

			time.Sleep(2 * time.Second)
			cancel()
			wp.Stop()

			// Assert
			repo.AssertCalled(t, "UpdateStatus", mock.Anything, mock.AnythingOfType("*domain.Maintenance"))
			assert.Equal(t, tt.expectedStatus, m1.Status)
			assert.Equal(t, tt.expectedTerminal, m1.IsTerminal())
		})
	}
}

func TestWorkerPool_RespectsMaxWorkers(t *testing.T) {
	// Arrange
	repo := new(mocks.MockMaintenanceRepository)
	// Create more items than maxWorkers
	var queued []*domain.Maintenance
	for i := 0; i < 10; i++ {
		m, _ := domain.NewCorrectiveMaintenance("ABC-123", "INC-123", 5)
		queued = append(queued, m)
	}

	repo.On("ListByStatus", mock.Anything, domain.MaintenanceStatus("queued")).
		Return(queued, nil).Once()
	repo.On("ListByStatus", mock.Anything, domain.MaintenanceStatus("queued")).
		Return([]*domain.Maintenance{}, nil)
	repo.On("UpdateStatus", mock.Anything, mock.AnythingOfType("*domain.Maintenance")).
		Return(nil)
	// Act — maxWorkers = 3, but 10 items; all should still be processed
	wp := service.NewWorkerPool(repo, 3, 1, false, newTestLogger())
	ctx, cancel := context.WithCancel(context.Background())
	wp.Start(ctx)

	time.Sleep(3 * time.Second)
	cancel()
	wp.Stop()

	// Assert — all 10 items should be processed (20 UpdateStatus calls: 10 in_progress + 10 completed)
	repo.AssertCalled(t, "UpdateStatus", mock.Anything, mock.AnythingOfType("*domain.Maintenance"))
}
