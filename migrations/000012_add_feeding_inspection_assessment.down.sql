ALTER TABLE inspections
    DROP CONSTRAINT inspections_feeding_assessment_values_check;

ALTER TABLE inspections
    DROP COLUMN feeding_need,
    DROP COLUMN feeding_performed,
    DROP COLUMN feed_types;
