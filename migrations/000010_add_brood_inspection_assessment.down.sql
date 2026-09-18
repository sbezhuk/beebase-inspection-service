ALTER TABLE inspections
    DROP CONSTRAINT inspections_brood_assessment_values_check,
    DROP COLUMN brood_amount,
    DROP COLUMN brood_pattern,
    DROP COLUMN brood_stages,
    DROP COLUMN brood_concerns;

