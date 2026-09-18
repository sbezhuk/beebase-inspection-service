-- HEALTH assessment v1 records observations only; it does not diagnose disease.
-- NULL arrays mean not assessed; empty arrays mean assessed and none observed.
ALTER TABLE inspections
    ADD COLUMN health_overall_condition TEXT,
    ADD COLUMN health_pest_signs TEXT[],
    ADD COLUMN health_warning_signs TEXT[],
    ADD COLUMN health_concern_level TEXT;

ALTER TABLE inspections
    ADD CONSTRAINT inspections_health_assessment_values_check
        CHECK (
            (health_overall_condition IS NULL OR health_overall_condition IN ('GOOD', 'FAIR', 'POOR')) AND
            (health_pest_signs IS NULL OR health_pest_signs <@ ARRAY['VARROA_MITES', 'WAX_MOTH', 'SMALL_HIVE_BEETLE', 'OTHER']::TEXT[]) AND
            (health_warning_signs IS NULL OR health_warning_signs <@ ARRAY['ABNORMAL_BROOD', 'DEFORMED_WINGS', 'UNUSUAL_BEE_MORTALITY', 'DIARRHEA_SIGNS', 'OTHER']::TEXT[]) AND
            (health_concern_level IS NULL OR health_concern_level IN ('NONE', 'LOW', 'MODERATE', 'HIGH'))
        );
