// Package domain contains the core business entities and rules for the
// FleetOps maintenance microservice. This package belongs to the Business
// Logic Layer and must have ZERO external infrastructure dependencies.
//
// SAD Reference: "Capa de lógica de negocio"
// Pattern: Domain Model (PoEAA)
package domain

import "errors"

// Sentinel errors for domain validation.
// These errors represent business rule violations and are used
// across the domain and service layers.
var (
	// ErrInvalidVehicleID indicates a nil or empty vehicle ID was provided.
	ErrInvalidVehicleID = errors.New("invalid vehicle ID: must not be empty")

	// ErrInvalidIncidentID indicates a nil or empty incident ID was provided.
	ErrInvalidIncidentID = errors.New("invalid incident ID: must not be empty")

	// ErrInvalidSeverity indicates the severity value is outside the valid range [1, 10].
	ErrInvalidSeverity = errors.New("invalid severity: must be between 1 and 10")

	// ErrInvalidStatusTransition indicates an illegal state machine transition was attempted.
	ErrInvalidStatusTransition = errors.New("invalid status transition")

	// ErrMaintenanceNotFound indicates no maintenance record exists for the given ID.
	ErrMaintenanceNotFound = errors.New("maintenance not found")

	// ErrInvalidMaintenanceType indicates an unrecognized maintenance type was provided.
	ErrInvalidMaintenanceType = errors.New("invalid maintenance type")
)
