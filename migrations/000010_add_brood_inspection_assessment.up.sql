-- BROOD assessment v1. NULL means not assessed; an empty array means
-- assessed and no brood stages were observed. TEXT[] keeps each stage
-- individually queryable (for example, brood_stages @> ARRAY['CAPPED']).
ALTER TABLE inspections
    ADD COLUMN brood_amount TEXT,
    ADD COLUMN brood_pattern TEXT,
    ADD COLUMN brood_stages TEXT[],
    ADD COLUMN brood_concerns TEXT;

ALTER TABLE inspections
    ADD CONSTRAINT inspections_brood_assessment_values_check
        CHECK (
            (brood_amount IS NULL OR brood_amount IN ('LOW', 'MODERATE', 'HIGH')) AND
            (brood_pattern IS NULL OR brood_pattern IN ('SOLID', 'MIXED', 'SPOTTY')) AND
            (brood_stages IS NULL OR brood_stages <@ ARRAY['EGGS', 'LARVAE', 'CAPPED']::TEXT[]) AND
            (brood_concerns IS NULL OR brood_concerns IN ('NONE', 'OBSERVED', 'UNSURE'))
        );

