-- Align inspection type values with this project's UPPER_SNAKE_CASE
-- convention for enum wire values (e.g. 'routine' -> 'ROUTINE').
ALTER TABLE inspections DROP CONSTRAINT inspections_type_check;

UPDATE inspections SET type = upper(type);

ALTER TABLE inspections ALTER COLUMN type SET DEFAULT 'ROUTINE';

ALTER TABLE inspections
    ADD CONSTRAINT inspections_type_check
        CHECK (type IN ('ROUTINE', 'QUEEN', 'BROOD', 'HEALTH', 'FEEDING', 'SEASONAL'));
