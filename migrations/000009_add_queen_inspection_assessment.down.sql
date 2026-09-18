ALTER TABLE inspections
    DROP CONSTRAINT inspections_queen_assessment_values_check,
    DROP COLUMN queen_observed,
    DROP COLUMN eggs_observed,
    DROP COLUMN queen_cells,
    DROP COLUMN queen_condition;

