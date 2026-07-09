package handler_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fleetops/maintenance/internal/domain"
	"github.com/fleetops/maintenance/internal/handler"
	"github.com/fleetops/maintenance/internal/handler/dto"
	"github.com/fleetops/maintenance/internal/mocks"
	"github.com/fleetops/maintenance/internal/service"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func setupHandler() (*handler.MaintenanceHandler, *mocks.MockMaintenanceRepository, *mocks.MockEventPublisher) {
	repo := new(mocks.MockMaintenanceRepository)
	publisher := new(mocks.MockEventPublisher)
	vehicleClient := new(mocks.MockVehicleFetcher)
	logger := testLogger()
	correctiveSvc := service.NewCorrectiveMaintenanceService(repo, publisher, logger)
	queueSvc := service.NewQueueService(repo, publisher, logger)
	thresholds := map[string]float64{"Automovil": 10000}
	preventiveSvc := service.NewPreventiveMaintenanceService(repo, vehicleClient, publisher, thresholds, 90, 1, logger)
	h := handler.NewMaintenanceHandler(correctiveSvc, queueSvc, preventiveSvc, logger)
	return h, repo, publisher
}

// =============================================================================

// ListAll handler tests
// =============================================================================

func TestListAll_Handler_Success(t *testing.T) {
	// Arrange
	h, repo, _ := setupHandler()

	items := []*domain.Maintenance{
		{ID: uuid.New(), VehicleID: "ABC-123", Type: domain.MaintenanceTypeCorrective, Status: domain.MaintenanceStatusQueued},
	}
	repo.On("List", mock.Anything).Return(items, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mantenimientos", nil)
	rr := httptest.NewRecorder()

	// Act
	h.ListAll(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp []*dto.MaintenanceResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Len(t, resp, 1)
}

func TestListAll_Handler_RepositoryError(t *testing.T) {
	// Arrange
	h, repo, _ := setupHandler()

	repo.On("List", mock.Anything).Return(nil, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mantenimientos", nil)
	rr := httptest.NewRecorder()

	// Act
	h.ListAll(rr, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// GetByID handler tests
// =============================================================================

func TestGetByID_Handler_Success(t *testing.T) {
	// Arrange
	h, repo, _ := setupHandler()

	id := uuid.New()
	expected := &domain.Maintenance{ID: id, VehicleID: "ABC-123", Status: domain.MaintenanceStatusQueued}
	repo.On("GetByID", mock.Anything, id).Return(expected, nil)

	// Use chi context to inject URL param
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mantenimientos/"+id.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// Simpler: set chi URL param directly
	rr := httptest.NewRecorder()

	// Act
	h.GetByID(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetByID_Handler_InvalidUUID(t *testing.T) {
	// Arrange
	h, _, _ := setupHandler()

	req := setChiURLParam(
		httptest.NewRequest(http.MethodGet, "/api/v1/mantenimientos/not-a-uuid", nil),
		"id", "not-a-uuid",
	)
	rr := httptest.NewRecorder()

	// Act
	h.GetByID(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetByID_Handler_NotFound(t *testing.T) {
	// Arrange
	h, repo, _ := setupHandler()

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, domain.ErrMaintenanceNotFound)

	req := setChiURLParam(
		httptest.NewRequest(http.MethodGet, "/api/v1/mantenimientos/"+id.String(), nil),
		"id", id.String(),
	)
	rr := httptest.NewRecorder()

	// Act
	h.GetByID(rr, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// setChiURLParam is a test helper to inject chi URL parameters.
func setChiURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// =============================================================================
// Finalize handler tests
// =============================================================================

func TestFinalize_Handler_Success(t *testing.T) {
	h, repo, publisher := setupHandler()

	id := uuid.New()
	m := &domain.Maintenance{ID: id, Status: domain.MaintenanceStatusInProgress, VehicleID: "XYZ-123"}

	repo.On("GetByID", mock.Anything, id).Return(m, nil)
	repo.On("UpdateStatus", mock.Anything, m).Return(nil)
	publisher.On("PublishMaintenanceEvent", mock.Anything, m, "COMPLETED").Return(nil)

	req := setChiURLParam(
		httptest.NewRequest(http.MethodPatch, "/api/v1/mantenimientos/"+id.String()+"/finalizar", nil),
		"id", id.String(),
	)
	rr := httptest.NewRecorder()

	h.Finalize(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestFinalize_Handler_NotFound(t *testing.T) {
	h, repo, _ := setupHandler()

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, domain.ErrMaintenanceNotFound)

	req := setChiURLParam(
		httptest.NewRequest(http.MethodPatch, "/api/v1/mantenimientos/"+id.String()+"/finalizar", nil),
		"id", id.String(),
	)
	rr := httptest.NewRecorder()

	h.Finalize(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// =============================================================================
// Router Integration test with JWT
// =============================================================================
func TestFinalize_Router_WithJWT(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"sub": "test"})
	tokenString, err := token.SignedString(privateKey)
	assert.NoError(t, err)

	h, repo, publisher := setupHandler()
	router := handler.NewRouter(h, handler.NewHealthHandler(nil), testLogger(), false, &privateKey.PublicKey, "RS256", false)

	id := uuid.New()
	m := &domain.Maintenance{ID: id, Status: domain.MaintenanceStatusInProgress, VehicleID: "XYZ-123"}

	repo.On("GetByID", mock.Anything, id).Return(m, nil)
	repo.On("UpdateStatus", mock.Anything, m).Return(nil)
	publisher.On("PublishMaintenanceEvent", mock.Anything, m, "COMPLETED").Return(nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/mantenimientos/"+id.String()+"/finalizar", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}
