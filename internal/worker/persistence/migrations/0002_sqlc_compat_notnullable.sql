-- GORM automigration wasn't smart, and thus the database had more nullable
-- columns than necessary. This migration makes columns that should never be
-- NULL actually NOT NULL.
--
-- Since this migration recreates all tables in the database, this is now also
-- done in a way that makes the schema more compatible with sqlc (which is
-- mostly removing various quotes and backticks, and replacing char(N) with
-- varchar(N)). sqlc is the tool that replaced GORM.
--
-- +goose Up
CREATE TABLE temp_task_updates (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  task_id varchar(36) DEFAULT '' NOT NULL,
  payload BLOB,
  PRIMARY KEY (id)
);
INSERT INTO temp_task_updates
  SELECT id, created_at, task_id, payload FROM task_updates;
DROP TABLE task_updates;
ALTER TABLE temp_task_updates RENAME TO task_updates;

-- +goose Down
CREATE TABLE IF NOT EXISTS `temp_task_updates` (
  `id` integer,
  `created_at` datetime,
  `task_id` varchar(36) DEFAULT "",
  `payload` BLOB,
  PRIMARY KEY (`id`)
);
INSERT INTO temp_task_updates
  SELECT id, created_at, task_id, payload FROM task_updates;
DROP TABLE task_updates;
ALTER TABLE temp_task_updates RENAME TO task_updates;
