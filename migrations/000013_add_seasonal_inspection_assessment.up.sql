ALTER TABLE inspections
    ADD COLUMN season TEXT,
    ADD COLUMN seasonal_store_readiness TEXT,
    ADD COLUMN seasonal_readiness TEXT,
    ADD COLUMN seasonal_concerns TEXT[];

ALTER TABLE inspections
    ADD CONSTRAINT inspections_seasonal_values_check CHECK (
        season IS NULL OR season IN ('SPRING', 'SUMMER', 'AUTUMN', 'WINTER')
    ),
    ADD CONSTRAINT inspections_seasonal_store_readiness_check CHECK (
        seasonal_store_readiness IS NULL OR seasonal_store_readiness IN ('SUFFICIENT', 'MARGINAL', 'INSUFFICIENT')
    ),
    ADD CONSTRAINT inspections_seasonal_readiness_check CHECK (
        seasonal_readiness IS NULL OR seasonal_readiness IN ('READY', 'NEEDS_ATTENTION', 'NOT_READY', 'UNSURE')
    ),
    ADD CONSTRAINT inspections_seasonal_concerns_check CHECK (
        seasonal_concerns IS NULL OR seasonal_concerns <@ ARRAY[
            'FOOD_STORES', 'COLONY_STRENGTH', 'QUEEN', 'BROOD',
            'PESTS_OR_DISEASE', 'HIVE_CONDITION', 'OTHER'
        ]::TEXT[]
    );

COMMENT ON COLUMN inspections.seasonal_concerns IS
    'Nullable array: NULL means not assessed; an empty array means assessed with no concerns identified.';
