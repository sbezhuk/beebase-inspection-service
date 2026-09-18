-- Nullable typed columns preserve legacy inspections and distinguish an
-- unanswered metric from a negative observation.
ALTER TABLE inspections
    ADD COLUMN assessment_version INTEGER,
    ADD COLUMN colony_strength TEXT,
    ADD COLUMN queen_status TEXT,
    ADD COLUMN brood_status TEXT,
    ADD COLUMN food_stores TEXT,
    ADD COLUMN health_concerns TEXT;

ALTER TABLE inspections
    ADD CONSTRAINT inspections_assessment_version_check
        CHECK (assessment_version IS NULL OR assessment_version = 1),
    ADD CONSTRAINT inspections_assessment_values_check
        CHECK (
            (colony_strength IS NULL OR colony_strength IN ('WEAK', 'MODERATE', 'STRONG')) AND
            (queen_status IS NULL OR queen_status IN ('HEALTHY', 'PROBLEM', 'NOT_CHECKED')) AND
            (brood_status IS NULL OR brood_status IN ('HEALTHY', 'PROBLEM', 'NOT_CHECKED')) AND
            (food_stores IS NULL OR food_stores IN ('LOW', 'ADEQUATE', 'ABUNDANT', 'NOT_CHECKED')) AND
            (health_concerns IS NULL OR health_concerns IN ('NONE', 'PRESENT', 'NOT_CHECKED'))
        );
