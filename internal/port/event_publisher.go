package port

import (
	"context"

	"github.com/fleetops/maintenance/internal/domain"
)

// EventPublisher defines the contract for emitting domain events
// to external messaging systems (e.g., SQS, Kafka).
//
// Pattern: Hexagonal Architecture (Driven Port), Event-Driven Architecture
type EventPublisher interface {
	PublishMaintenanceEvent(ctx context.Context, m *domain.Maintenance, status string) error
}
