package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/fleetops/maintenance/internal/domain"
)

// MockMaintenanceRepository is a testify mock implementing port.MaintenanceRepository.
// Generated manually to match mockery output format.
type MockMaintenanceRepository struct {
	mock.Mock
}

func (m *MockMaintenanceRepository) Create(ctx context.Context, maintenance *domain.Maintenance) error {
	return m.Called(ctx, maintenance).Error(0)
}

func (m *MockMaintenanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Maintenance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Maintenance), args.Error(1)
}

func (m *MockMaintenanceRepository) List(ctx context.Context) ([]*domain.Maintenance, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Maintenance), args.Error(1)
}

func (m *MockMaintenanceRepository) ListByStatus(ctx context.Context, status domain.MaintenanceStatus) ([]*domain.Maintenance, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Maintenance), args.Error(1)
}

func (m *MockMaintenanceRepository) UpdateStatus(ctx context.Context, maintenance *domain.Maintenance) error {
	return m.Called(ctx, maintenance).Error(0)
}

func (m *MockMaintenanceRepository) ListOldUncompleted(ctx context.Context, thresholdMinutes int) ([]*domain.Maintenance, error) {
	args := m.Called(ctx, thresholdMinutes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Maintenance), args.Error(1)
}
