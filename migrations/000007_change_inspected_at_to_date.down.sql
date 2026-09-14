-- Restore the previous timestamp representation at midnight UTC.
ALTER TABLE inspections
    ALTER COLUMN inspected_at TYPE TIMESTAMPTZ
    USING inspected_at::timestamp AT TIME ZONE 'UTC';
