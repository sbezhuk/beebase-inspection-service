-- DEFAULT 'routine' backfills existing rows; every new row goes through
-- the application layer, which always supplies an explicit, validated
-- type, so the default is only ever exercised by this migration itself.
ALTER TABLE inspections
    ADD COLUMN type TEXT NOT NULL DEFAULT 'routine'
        CHECK (type IN ('routine', 'queen', 'brood', 'health', 'feeding', 'seasonal'));
