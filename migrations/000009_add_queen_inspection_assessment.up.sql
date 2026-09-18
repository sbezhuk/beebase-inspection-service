-- QUEEN assessment v1 fields. Nullable columns preserve legacy inspections
-- and distinguish omitted observations from explicit enum values.
ALTER TABLE inspections
    ADD COLUMN queen_observed TEXT,
    ADD COLUMN eggs_observed TEXT,
    ADD COLUMN queen_cells TEXT,
    ADD COLUMN queen_condition TEXT;

ALTER TABLE inspections
    ADD CONSTRAINT inspections_queen_assessment_values_check
        CHECK (
            (queen_observed IS NULL OR queen_observed IN ('OBSERVED', 'NOT_OBSERVED', 'UNSURE')) AND
            (eggs_observed IS NULL OR eggs_observed IN ('YES', 'NO', 'UNSURE')) AND
            (queen_cells IS NULL OR queen_cells IN ('NONE', 'PRESENT', 'UNSURE')) AND
            (queen_condition IS NULL OR queen_condition IN ('NORMAL', 'CONCERN'))
        );

