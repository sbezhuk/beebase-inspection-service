-- Sorting by creation date (?sortOrder=asc|desc) is now a first-class
-- option alongside the default InspectedAt order, which
-- idx_inspections_user_hive_inspected_at already serves. Add the
-- equivalent composite index on created_at so that request path performs
-- just as well as the default one.
CREATE INDEX idx_inspections_user_hive_created_at ON inspections (user_id, hive_id, created_at, id) WHERE deleted_at IS NULL;
