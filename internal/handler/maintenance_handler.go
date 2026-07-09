package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fleetops/maintenance/internal/domain"
	"github.com/fleetops/maintenance/internal/handler/dto"
	"github.com/fleetops/maintenance/internal/service"
)

// MaintenanceHandler handles HTTP requests for maintenance operations.
//
// SAD Reference: "HTTP Handler valida los datos recibidos"
// Pattern: Handler (Presentation Layer)
type MaintenanceHandler struct {
	correctiveSvc *service.CorrectiveMaintenanceService
	queueSvc      *service.QueueService
	logger        *slog.Logger
}

// NewMaintenanceHandler constructs a MaintenanceHandler with injected services.
func NewMaintenanceHandler(
	correctiveSvc *service.CorrectiveMaintenanceService,
	queueSvc *service.QueueService,
	logger *slog.Logger,
) *MaintenanceHandler {
	return &MaintenanceHandler{
		correctiveSvc: correctiveSvc,
		queueSvc:      queueSvc,
		logger:        logger,
	}
}

// ListAll handles GET /api/v1/mantenimientos
//
// SAD Reference: Process Network 3 — "Consulta de Cola de Mantenimientos"
func (h *MaintenanceHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	items, err := h.queueSvc.ListAll(r.Context())
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "list_failed", "Failed to retrieve maintenances")
		return
	}

	h.respondJSON(w, http.StatusOK, dto.FromDomainList(items))
}

// GetByID handles GET /api/v1/mantenimientos/{id}
func (h *MaintenanceHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "ID must be a valid UUID")
		return
	}

	m, err := h.queueSvc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrMaintenanceNotFound) {
			h.respondError(w, http.StatusNotFound, "not_found", "Maintenance not found")
			return
		}
		h.respondError(w, http.StatusInternalServerError, "get_failed", "Failed to retrieve maintenance")
		return
	}

	h.respondJSON(w, http.StatusOK, dto.FromDomain(m))
}

// Finalize handles PATCH /api/v1/mantenimientos/{id}/finalizar
// Marks the maintenance as completed and publishes to SQS.
func (h *MaintenanceHandler) Finalize(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "ID must be a valid UUID")
		return
	}

	if err := h.queueSvc.FinalizeMaintenance(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrMaintenanceNotFound) {
			h.respondError(w, http.StatusNotFound, "not_found", "Maintenance not found")
			return
		}
		h.respondError(w, http.StatusInternalServerError, "finalize_failed", "Failed to finalize maintenance")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetQueueSummary handles GET /api/v1/mantenimientos/cola
// Returns the maintenance queue summary: queued + in-progress items.
//
// SAD Reference: Process Network 3 — "listado de vehículos: en cola, en mantenimiento"
func (h *MaintenanceHandler) GetQueueSummary(w http.ResponseWriter, r *http.Request) {
	queued, err := h.queueSvc.ListQueued(r.Context())
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "queue_failed", "Failed to retrieve queue")
		return
	}

	inProgress, err := h.queueSvc.ListInProgress(r.Context())
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "queue_failed", "Failed to retrieve in-progress")
		return
	}

	summary := dto.QueueSummaryResponse{
		Queued:      dto.FromDomainList(queued),
		InProgress:  dto.FromDomainList(inProgress),
		TotalQueued: len(queued),
		TotalActive: len(inProgress),
	}

	h.respondJSON(w, http.StatusOK, summary)
}

// GetReport handles GET /api/v1/mantenimientos/reporte
// Returns a simplified list of maintenances for reporting purposes.
func (h *MaintenanceHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	items, err := h.queueSvc.ListAll(r.Context())
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "report_failed", "Failed to retrieve maintenances for report")
		return
	}

	report := make([]dto.ReportResponse, 0, len(items))
	for _, m := range items {
		// Map status: true = in_progress (en mant), false = queued (programado)
		// For other statuses, we can default to false or handle them if needed.
		estadoMant := m.IsInProgress()

		// Map severity: if > 0 it's GRAVE (corrective), else LEVE (preventive)
		gravedad := "LEVE"
		if m.Severity > 0 {
			gravedad = "GRAVE"
		}

		// Map date: use CreatedAt as the scheduled/maintenance date
		fechaMant := m.CreatedAt.Format("2006-01-02T15:04:05Z")

		report = append(report, dto.ReportResponse{
			VehicleID:         m.VehicleID,
			MaintenanceDate:   fechaMant,
			MaintenanceStatus: estadoMant,
			Severity:          gravedad,
		})
	}

	h.respondJSON(w, http.StatusOK, report)
}

// respondJSON writes a JSON response with the given status code.
func (h *MaintenanceHandler) respondJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", slog.String("error", err.Error()))
	}
}

// respondError writes a structured error response.
func (h *MaintenanceHandler) respondError(w http.ResponseWriter, code int, errType, message string) {
	h.respondJSON(w, code, dto.ErrorResponse{
		Error:   errType,
		Message: message,
		Code:    code,
	})
}
