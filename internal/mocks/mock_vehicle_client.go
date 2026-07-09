package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/fleetops/maintenance/internal/domain"
)

// MockVehicleFetcher is a testify mock implementing port.VehicleFetcher.
// Generated manually to match mockery output format.
type MockVehicleFetcher struct {
	mock.Mock
}

func (m *MockVehicleFetcher) GetAllVehicles(ctx context.Context) ([]*domain.Vehicle, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Vehicle), args.Error(1)
}
