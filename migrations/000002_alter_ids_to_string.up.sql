-- =============================================================================
-- Migration: Alter ID columns to VARCHAR
-- Description: Changes vehicle_id and incident_id from UUID to VARCHAR to 
-- support string-based identifiers from external services.
-- =============================================================================

ALTER TABLE maintenances 
    ALTER COLUMN vehicle_id TYPE VARCHAR(255) USING vehicle_id::text,
    ALTER COLUMN incident_id TYPE VARCHAR(255) USING incident_id::text;
