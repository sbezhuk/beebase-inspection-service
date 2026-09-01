ALTER TABLE inspections DROP CONSTRAINT inspections_type_check;

UPDATE inspections SET type = lower(type);

ALTER TABLE inspections ALTER COLUMN type SET DEFAULT 'routine';

ALTER TABLE inspections
    ADD CONSTRAINT inspections_type_check
        CHECK (type IN ('routine', 'queen', 'brood', 'health', 'feeding', 'seasonal'));
