-- Drop tables that were in use in beta versions of Flamenco. These might exist
-- in developer databases, as well as databases of studios following the `main`
-- branch, such as Blender Studio.
--
-- WARNING: this migration simply drops the tables. Their data is erased, and
-- cannot be brought back by rolling the migration back.
--
-- +goose Up
DROP INDEX IF EXISTS `idx_worker_clusters_uuid`;
DROP TABLE IF EXISTS `worker_cluster_membership`;
DROP TABLE IF EXISTS `worker_clusters`;

-- +goose Down
-- Do not recreate these tables, as no release of Flamenco ever used them.
-- Also their contents wouldn't be brought back anyway.
