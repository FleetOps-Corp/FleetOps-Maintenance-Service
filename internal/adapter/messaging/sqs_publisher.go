package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/fleetops/maintenance/internal/domain"
	"github.com/fleetops/maintenance/internal/port"
)

var _ port.EventPublisher = (*SQSPublisher)(nil)

// SQSPublisher implements port.EventPublisher for AWS SQS.
type SQSPublisher struct {
	client          *sqs.Client
	queueURL        string
	logger          *slog.Logger
	useMockFallback bool
}

func NewSQSPublisher(client *sqs.Client, queueURL string, useMockFallback bool, logger *slog.Logger) *SQSPublisher {
	return &SQSPublisher{
		client:          client,
		queueURL:        queueURL,
		logger:          logger,
		useMockFallback: useMockFallback,
	}
}

type MaintenanceSQSEvent struct {
	MaintenanceID   string `json:"maintenanceId"`
	VehicleID       string `json:"vehicleId"`
	MaintenanceType string `json:"maintenanceType"`
	Status          string `json:"status"`
	OccurredAt      string `json:"occurredAt"`
}

func (p *SQSPublisher) PublishMaintenanceEvent(ctx context.Context, m *domain.Maintenance, status string) error {
	event := MaintenanceSQSEvent{
		MaintenanceID:   m.ID.String(),
		VehicleID:       m.VehicleID, // Mapeo estricto de la Placa solicitada
		MaintenanceType: string(m.Type),
		Status:          status,
		OccurredAt:      time.Now().Format(time.RFC3339),
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal sqs event: %w", err)
	}

	input := &sqs.SendMessageInput{
		MessageBody: aws.String(string(body)),
		QueueUrl:    aws.String(p.queueURL),
	}

	_, err = p.client.SendMessage(ctx, input)
	if err != nil {
		p.logger.ErrorContext(
			ctx, "failed to publish message to SQS",
			slog.String("queueUrl", p.queueURL),
			slog.String("maintenanceId", event.MaintenanceID),
			slog.String("error", err.Error()),
		)
		if p.useMockFallback {
			p.logger.WarnContext(
				ctx, "USE_MOCK_FALLBACK enabled: simulating SQS publish success",
				slog.String("event_body", string(body)),
			)
			return nil
		}
		return fmt.Errorf("sqs send message: %w", err)
	}

	p.logger.InfoContext(
		ctx, "published maintenance event to SQS",
		slog.String("maintenanceId", event.MaintenanceID),
		slog.String("status", status),
	)

	return nil
}
