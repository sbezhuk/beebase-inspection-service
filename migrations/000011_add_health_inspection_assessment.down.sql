ALTER TABLE inspections
    DROP CONSTRAINT inspections_health_assessment_values_check;

ALTER TABLE inspections
    DROP COLUMN health_overall_condition,
    DROP COLUMN health_pest_signs,
    DROP COLUMN health_warning_signs,
    DROP COLUMN health_concern_level;
