package messaging

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/fleetops/maintenance/internal/service"
)

// IncidentEvent represents the payload published by the Incidents microservice.
type IncidentEvent struct {
	IncidentID   string `json:"incident_id"`
	DriverID     string `json:"driver_id"`
	VehicleID    string `json:"vehicle_id"`
	IncidentType string `json:"incident_type"`
	Severity     string `json:"severity"`
	Description  string `json:"description"`
	EventDate    string `json:"event_date"`
}

// SQSConsumer is an adapter that listens to an SQS queue for incoming
// incident events and forwards relevant ones to the CorrectiveMaintenanceService.
type SQSConsumer struct {
	sqsClient     *sqs.Client
	queueURL      string
	correctiveSvc *service.CorrectiveMaintenanceService
	logger        *slog.Logger
	stopCh        chan struct{}
	stopped       sync.Once
}

// NewSQSConsumer creates a new SQSConsumer.
func NewSQSConsumer(client *sqs.Client, queueURL string, correctiveSvc *service.CorrectiveMaintenanceService, logger *slog.Logger) *SQSConsumer {
	return &SQSConsumer{
		sqsClient:     client,
		queueURL:      queueURL,
		correctiveSvc: correctiveSvc,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}
}

// Start begins polling the SQS queue for messages in the background.
func (c *SQSConsumer) Start(ctx context.Context) {
	c.logger.Info("starting SQS consumer", slog.String("queue_url", c.queueURL))
	go c.poll(ctx)
}

// Stop signals the consumer to stop polling.
func (c *SQSConsumer) Stop() {
	c.stopped.Do(func() {
		close(c.stopCh)
	})
}

func (c *SQSConsumer) poll(ctx context.Context) {
	for {
		select {
		case <-c.stopCh:
			c.logger.Info("SQS consumer stopped")
			return
		case <-ctx.Done():
			c.logger.Info("SQS consumer context cancelled")
			return
		default:
			// Continue polling
		}

		// Receive messages with Long Polling (20s)
		input := &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
		}

		output, err := c.sqsClient.ReceiveMessage(ctx, input)
		if err != nil {
			c.logger.Error("failed to receive messages from SQS", slog.String("error", err.Error()))
			time.Sleep(5 * time.Second) // backoff
			continue
		}

		for _, msg := range output.Messages {
			c.processMessage(ctx, msg)
		}
	}
}

func (c *SQSConsumer) processMessage(ctx context.Context, msg types.Message) {
	if msg.Body == nil {
		return
	}

	var event IncidentEvent
	if err := json.Unmarshal([]byte(*msg.Body), &event); err != nil {
		c.logger.WarnContext(ctx, "failed to parse incident event JSON", slog.String("error", err.Error()), slog.String("body", *msg.Body))
		c.deleteMessage(ctx, msg.ReceiptHandle)
		return
	}

	// Filtrado: solo procesamos MECANICO y GRAVE
	if event.IncidentType != "MECANICO" || event.Severity != "GRAVE" {
		c.logger.InfoContext(ctx, "skipping irrelevant incident",
			slog.String("incident_id", event.IncidentID),
			slog.String("type", event.IncidentType),
			slog.String("severity", event.Severity))
		c.deleteMessage(ctx, msg.ReceiptHandle)
		return
	}

	c.logger.InfoContext(ctx, "processing grave mechanic incident", slog.String("incident_id", event.IncidentID), slog.String("vehicle_id", event.VehicleID))

	// Como es grave, asignamos aleatoriamente gravedad del 1 al 10 para determinar cuánto tiempo tardará
	randomSeverity := uint8(rand.Intn(10) + 1)

	// Crear el mantenimiento correctivo
	_, err := c.correctiveSvc.CreateCorrective(ctx, event.VehicleID, event.IncidentID, randomSeverity)
	if err != nil {
		c.logger.ErrorContext(ctx, "failed to create corrective maintenance from SQS", slog.String("error", err.Error()))
		// No eliminamos el mensaje para que vuelva a la cola (DLQ eventual)
		return
	}

	// Eliminar el mensaje al procesar con éxito
	c.deleteMessage(ctx, msg.ReceiptHandle)
}

func (c *SQSConsumer) deleteMessage(ctx context.Context, receiptHandle *string) {
	if receiptHandle == nil {
		return
	}
	_, err := c.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: receiptHandle,
	})
	if err != nil {
		c.logger.ErrorContext(ctx, "failed to delete message from SQS", slog.String("error", err.Error()))
	}
}
