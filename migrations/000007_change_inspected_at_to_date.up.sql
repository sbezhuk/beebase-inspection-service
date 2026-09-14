-- Inspections are recorded for a calendar day, not a time instant. Preserve
-- the existing UTC calendar date while removing the time-of-day component.
ALTER TABLE inspections
    ALTER COLUMN inspected_at TYPE DATE
    USING (inspected_at AT TIME ZONE 'UTC')::date;
