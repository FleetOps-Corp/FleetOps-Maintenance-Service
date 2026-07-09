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

// Compile-time check: HTTPVehicleClient implements port.VehicleClient.
var _ port.VehicleClient = (*HTTPVehicleClient)(nil)

// HTTPVehicleClient is the concrete implementation of port.VehicleClient
// that communicates with the external Vehicles microservice via REST/HTTP.
//
// This adapter implements the Anti-Corruption Layer (ACL), translating
// external API responses into domain.Vehicle value objects.
//
// [Archetype Convention Addition] — Anti-Corruption Layer (DDD best practice)
// SAD Reference: Process Network 2 — "GET /vehiculos", "PUT /vehiculos"
type HTTPVehicleClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
	log        *slog.Logger
}

// NewHTTPVehicleClient constructs an HTTPVehicleClient.
func NewHTTPVehicleClient(baseURL, token string, timeoutSecs int, log *slog.Logger) *HTTPVehicleClient {
	return &HTTPVehicleClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSecs) * time.Second,
		},
		log: log,
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

// GetAllVehicles fetches all vehicles from the Vehicles microservice and
// translates them into domain.Vehicle value objects.
//
// ACL Translation: externalVehicle → domain.Vehicle
func (c *HTTPVehicleClient) GetAllVehicles(ctx context.Context) ([]*domain.Vehicle, error) {
	url := fmt.Sprintf("%s/vehiculos/disponibles", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating vehicle list request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing vehicle list request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vehicle service returned status %d", resp.StatusCode)
	}

	var wrapper vehiclesResponse
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("decoding vehicle list response: %w", err)
	}

	// ACL Translation: convert external models to domain models
	vehicles := make([]*domain.Vehicle, 0, len(wrapper.Content))
	for _, ev := range wrapper.Content {
		if ev.IdVehiculo == "" {
			continue // skip vehicles with invalid IDs
		}

		// 1. Independent parse for CreatedAt
		var createdAt time.Time
		if ev.CreadoEn != "" {
			if parsed, err := parseDateSafe(ev.CreadoEn); err == nil {
				createdAt = parsed
			}
		}

		// 2. Mathematical calculation for daysSince (Optimized)
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
			// Fallback: reutilizamos el createdAt ya parseado (Cero costo de CPU extra)
			daysSince = int(time.Since(createdAt).Hours() / 24)
		} else {
			c.log.WarnContext(
				ctx, "sin fechas validas para obtener dias, asumiendo 0",
				slog.String("placa", ev.NumeroPlaca),
			)
		}

		vehicles = append(vehicles, &domain.Vehicle{
			ID:                       ev.NumeroPlaca, // The rest of our system treats VehicleID as the Placa
			LicensePlate:             ev.NumeroPlaca,
			VehicleType:              ev.TipoVehiculo,
			CreatedAt:                createdAt,
			KilometersRecorded:       ev.Kilometraje,
			DaysSinceLastMaintenance: daysSince,
			Available:                ev.EstadoVehiculo == "DISPONIBLE" && ev.Activo, // Map ENUM + Activo
		})
	}

	return vehicles, nil
}
