-- FEEDING assessment v1 separates observed food stores from beekeeper action.
-- NULL arrays mean not assessed; empty arrays mean feeding was performed but
-- no concrete feed type was recorded.
ALTER TABLE inspections
    ADD COLUMN feeding_need TEXT,
    ADD COLUMN feeding_performed TEXT,
    ADD COLUMN feed_types TEXT[];

ALTER TABLE inspections
    ADD CONSTRAINT inspections_feeding_assessment_values_check
        CHECK (
            (feeding_need IS NULL OR feeding_need IN ('NO', 'SOON', 'YES', 'UNSURE')) AND
            (feeding_performed IS NULL OR feeding_performed IN ('YES', 'NO')) AND
            (feed_types IS NULL OR feed_types <@ ARRAY['SUGAR_SYRUP', 'FONDANT', 'DRY_SUGAR', 'POLLEN_SUBSTITUTE', 'OTHER']::TEXT[]) AND
            (feeding_performed = 'YES' OR feed_types IS NULL OR cardinality(feed_types) = 0)
        );
