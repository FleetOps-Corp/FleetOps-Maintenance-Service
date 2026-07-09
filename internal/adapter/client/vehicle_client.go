package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/fleetops/maintenance/internal/domain"
	"github.com/fleetops/maintenance/internal/port"
)

// Compile-time check: HTTPVehicleClient implements port.VehicleFetcher.
var _ port.VehicleFetcher = (*HTTPVehicleClient)(nil)

// HTTPVehicleClient is the concrete implementation of port.VehicleFetcher
// that communicates with the external Vehicles microservice via REST/HTTP.
//
// This adapter implements the Anti-Corruption Layer (ACL), translating
// external API responses into domain.Vehicle value objects.
//
// [Archetype Convention Addition] — Anti-Corruption Layer (DDD best practice)
// SAD Reference: Process Network 2 — "GET /vehiculos", "PUT /vehiculos"
type HTTPVehicleClient struct {
	baseURL         string
	token           string
	httpClient      *http.Client
	log             *slog.Logger
	useMockFallback bool
}

// NewHTTPVehicleClient constructs an HTTPVehicleClient.
func NewHTTPVehicleClient(baseURL, token string, timeoutSecs int, useMockFallback bool, log *slog.Logger) *HTTPVehicleClient {
	return &HTTPVehicleClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSecs) * time.Second,
		},
		useMockFallback: useMockFallback,
		log:             log,
	}
}

func parseDateSafe(dateStr string) (time.Time, error) {
	if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("invalid date format")
}

// externalVehicle represents the JSON structure returned by the external
// Vehicles microservice. This is the "foreign" model that gets translated.
type externalVehicle struct {
	IdVehiculo      string  `json:"idVehiculo"`
	NumeroPlaca     string  `json:"numeroPlaca"`
	TipoVehiculo    string  `json:"nombreTipoVehiculo"`
	CreadoEn        string  `json:"creadoEn"`
	Kilometraje     float64 `json:"kilometraje"`
	FechaUltimoMant string  `json:"fechaUltimoMant"`
	EstadoVehiculo  string  `json:"estadoVehiculo"`
	Activo          bool    `json:"activo"`
}

type vehiclesResponse struct {
	Content []externalVehicle `json:"content"`
}

func (c *HTTPVehicleClient) fallbackOrError(ctx context.Context, err error, msg string) ([]*domain.Vehicle, error) {
	if c.useMockFallback {
		c.log.WarnContext(ctx, msg, slog.String("error", err.Error()))
		return c.getMockVehicles(), nil
	}
	return nil, err
}

// GetAllVehicles fetches all vehicles from the Vehicles microservice and
// translates them into domain.Vehicle value objects.
//
// ACL Translation: externalVehicle → domain.Vehicle
func (c *HTTPVehicleClient) GetAllVehicles(ctx context.Context) ([]*domain.Vehicle, error) {
	url := fmt.Sprintf("%s/vehiculos/disponibles", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return c.fallbackOrError(ctx, fmt.Errorf("creating vehicle list request: %w", err), "failed to create vehicle list request, returning mock vehicles (fallback)")
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return c.fallbackOrError(ctx, fmt.Errorf("executing vehicle list request: %w", err), "failed to execute vehicle list request, returning mock vehicles (fallback)")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errStatus := fmt.Errorf("vehicle service returned status %d", resp.StatusCode)
		return c.fallbackOrError(ctx, errStatus, "vehicle service returned non-200 status, returning mock vehicles (fallback)")
	}

	var wrapper vehiclesResponse
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return c.fallbackOrError(ctx, fmt.Errorf("decoding vehicle list response: %w", err), "failed to decode vehicle list response, returning mock vehicles (fallback)")
	}

	// ACL Translation: convert external models to domain models
	vehicles := make([]*domain.Vehicle, 0, len(wrapper.Content))
	for _, ev := range wrapper.Content {
		if ev.IdVehiculo != "" {
			vehicles = append(vehicles, c.mapVehicle(ctx, ev))
		}
	}

	return vehicles, nil
}

func (c *HTTPVehicleClient) getMockVehicles() []*domain.Vehicle {
	return []*domain.Vehicle{
		{
			ID:                       "ABC-123",
			LicensePlate:             "ABC-123",
			VehicleType:              "Automovil",
			CreatedAt:                time.Now().Add(-365 * 24 * time.Hour),
			KilometersRecorded:       12000,
			DaysSinceLastMaintenance: 95,
			Available:                true,
		},
		{
			ID:                       "XYZ-789",
			LicensePlate:             "XYZ-789",
			VehicleType:              "Camioneta",
			CreatedAt:                time.Now().Add(-180 * 24 * time.Hour),
			KilometersRecorded:       15000,
			DaysSinceLastMaintenance: 45,
			Available:                true,
		},
		{
			ID:                       "SPE-999",
			LicensePlate:             "SPE-999",
			VehicleType:              "Vehiculo_especializado",
			CreatedAt:                time.Now().Add(-720 * 24 * time.Hour),
			KilometersRecorded:       5000,
			DaysSinceLastMaintenance: 120,
			Available:                true,
		},
	}
}

func (c *HTTPVehicleClient) mapVehicle(ctx context.Context, ev externalVehicle) *domain.Vehicle {
	var createdAt time.Time
	if ev.CreadoEn != "" {
		if parsed, err := parseDateSafe(ev.CreadoEn); err == nil {
			createdAt = parsed
		}
	}

	daysSince := 0
	if ev.FechaUltimoMant != "" {
		if parsedDate, err := parseDateSafe(ev.FechaUltimoMant); err == nil {
			daysSince = int(time.Since(parsedDate).Hours() / 24)
		} else {
			c.log.WarnContext(
				ctx, "fecha de mantenimiento invalida, asumiendo 0 dias",
				slog.String("placa", ev.NumeroPlaca),
				slog.String("fecha", ev.FechaUltimoMant),
			)
		}
	} else if !createdAt.IsZero() {
		daysSince = int(time.Since(createdAt).Hours() / 24)
	} else {
		c.log.WarnContext(
			ctx, "sin fechas validas para obtener dias, asumiendo 0",
			slog.String("placa", ev.NumeroPlaca),
		)
	}

	return &domain.Vehicle{
		ID:                       ev.NumeroPlaca,
		LicensePlate:             ev.NumeroPlaca,
		VehicleType:              ev.TipoVehiculo,
		CreatedAt:                createdAt,
		KilometersRecorded:       ev.Kilometraje,
		DaysSinceLastMaintenance: daysSince,
		Available:                ev.EstadoVehiculo == "DISPONIBLE" && ev.Activo,
	}
}
