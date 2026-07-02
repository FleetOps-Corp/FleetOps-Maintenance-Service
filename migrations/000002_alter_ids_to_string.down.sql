-- =============================================================================
-- Migration: Revert alter ID columns to VARCHAR
-- =============================================================================

ALTER TABLE maintenances 
    ALTER COLUMN vehicle_id TYPE UUID USING vehicle_id::uuid,
    ALTER COLUMN incident_id TYPE UUID USING incident_id::uuid;
