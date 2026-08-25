CREATE TABLE inspections (
    id           UUID PRIMARY KEY,
    -- No foreign key to a hives table: hives live in a different service
    -- and database. hive_id is opaque here; ownership (and, transitively,
    -- apiary ownership) was confirmed against hive-service once, at
    -- creation time.
    hive_id      UUID NOT NULL,
    -- Denormalized owner, copied from the verified hive's owner at
    -- creation time. Immutable thereafter (hive_id never changes), so
    -- every later read/write can be scoped by user_id directly, without
    -- another cross-service call.
    user_id      UUID NOT NULL,
    inspected_at TIMESTAMPTZ NOT NULL,
    notes        TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

-- Serves ListByHive's "WHERE user_id = $1 AND hive_id = $2" directly.
-- GetByID/Update/Delete key off the primary key (id) plus this same
-- user_id filter, so no separate index is needed for those.
CREATE INDEX idx_inspections_user_hive ON inspections (user_id, hive_id) WHERE deleted_at IS NULL;
