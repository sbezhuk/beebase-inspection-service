DROP INDEX idx_inspections_user_hive_inspected_at;

CREATE INDEX idx_inspections_user_hive ON inspections (user_id, hive_id) WHERE deleted_at IS NULL;
