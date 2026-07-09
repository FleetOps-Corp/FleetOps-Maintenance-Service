package mocks

import (
	"context"

	"github.com/fleetops/maintenance/internal/domain"
	"github.com/stretchr/testify/mock"
)

// MockEventPublisher is a mock of port.EventPublisher
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) PublishMaintenanceEvent(ctx context.Context, maintenance *domain.Maintenance, status string) error {
	args := m.Called(ctx, maintenance, status)
	return args.Error(0)
}
