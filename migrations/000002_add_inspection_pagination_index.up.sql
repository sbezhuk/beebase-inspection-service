-- ListByHive now orders by (inspected_at, id) and paginates with
-- LIMIT/OFFSET; replace the (user_id, hive_id) index with a composite one
-- that covers the WHERE clause, the ORDER BY, and the COUNT(*) query used
-- for pagination metadata.
DROP INDEX idx_inspections_user_hive;

CREATE INDEX idx_inspections_user_hive_inspected_at ON inspections (user_id, hive_id, inspected_at, id) WHERE deleted_at IS NULL;
