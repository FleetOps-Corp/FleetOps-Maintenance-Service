package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
}

// NewHTTPVehicleClient constructs an HTTPVehicleClient.
func NewHTTPVehicleClient(baseURL, token string, timeoutSecs int) *HTTPVehicleClient {
	return &HTTPVehicleClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSecs) * time.Second,
		},
	}
}

// externalVehicle represents the JSON structure returned by the external
// Vehicles microservice. This is the "foreign" model that gets translated.
type externalVehicle struct {
	IdVehiculo      string  `json:"idVehiculo"`
	NumeroPlaca     string  `json:"numeroPlaca"`
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

		// Calculate days since last maintenance
		daysSince := 0
		if ev.FechaUltimoMant != "" {
			parsedDate, err := time.Parse("2006-01-02", ev.FechaUltimoMant)
			if err == nil {
				duration := time.Since(parsedDate)
				daysSince = int(duration.Hours() / 24)
			}
		}

		// isAvailable := ev.EstadoVehiculo == "DISPONIBLE" && ev.Activo // If strictly available

		vehicles = append(vehicles, &domain.Vehicle{
			ID:                       ev.NumeroPlaca, // The rest of our system treats VehicleID as the Placa
			LicensePlate:             ev.NumeroPlaca,
			KilometersRecorded:       ev.Kilometraje,
			DaysSinceLastMaintenance: daysSince,
			Available:                ev.EstadoVehiculo == "DISPONIBLE", // Map ENUM to our boolean
		})
	}

	return vehicles, nil
}

type vehicleUpdatePayload struct {
	NuevoEstado    string `json:"nuevoEstado"`
	MotivoCambio   string `json:"motivoCambio"`
	ServicioOrigen string `json:"servicioOrigen"`
}

// SAD Reference: "PATCH a /vehículos/{id}/estado — actualiza estado a EN_MANTENIMIENTO"
func (c *HTTPVehicleClient) UpdateVehicleMaintenanceStatus(ctx context.Context, vehicleID string) error {
	payload := vehicleUpdatePayload{
		NuevoEstado:    "EN_MANTENIMIENTO",
		MotivoCambio:   "reparacion del motor",
		ServicioOrigen: "microservicio-mantenimientos",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling vehicle update payload: %w", err)
	}

	// The team provided /vehiculos/placa/{placa}/estado, but we only have the VehicleID (UUID)
	// at this point in the worker pool. We will send the UUID and see if their backend accepts it.
	url := fmt.Sprintf("%s/vehiculos/placa/%s/estado", c.baseURL, vehicleID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating vehicle update request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing vehicle update request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("vehicle service returned status %d on update", resp.StatusCode)
	}

	return nil
}
