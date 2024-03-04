-- GORM automigration wasn't smart, and thus the database had more nullable
-- columns than necessary. This migration makes columns that should never be
-- NULL actually NOT NULL.
--
-- Since this migration recreates all tables in the database, this is now also
-- done in a way that makes the schema more compatible with sqlc (which is
-- mostly removing various quotes and backticks, and replacing char(N) with
-- varchar(N)). sqlc is the tool that'll replace GORM.
--
-- +goose Up
CREATE TABLE temp_last_rendereds (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  updated_at datetime,
  job_id integer DEFAULT 0 NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_last_rendereds_job FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);
INSERT INTO temp_last_rendereds
  SELECT id, created_at, updated_at, job_id FROM last_rendereds;
DROP TABLE last_rendereds;
ALTER TABLE temp_last_rendereds RENAME TO last_rendereds;

CREATE TABLE temp_task_dependencies (
  task_id integer NOT NULL,
  dependency_id integer NOT NULL,
  PRIMARY KEY (task_id, dependency_id),
  CONSTRAINT fk_task_dependencies_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
  CONSTRAINT fk_task_dependencies_dependencies FOREIGN KEY (dependency_id) REFERENCES tasks(id) ON DELETE CASCADE
);
INSERT INTO temp_task_dependencies SELECT task_id, dependency_id FROM task_dependencies;
DROP TABLE task_dependencies;
ALTER TABLE temp_task_dependencies RENAME TO task_dependencies;

CREATE TABLE temp_task_failures (
  created_at datetime NOT NULL,
  task_id integer NOT NULL,
  worker_id integer NOT NULL,
  PRIMARY KEY (task_id, worker_id),
  CONSTRAINT fk_task_failures_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
  CONSTRAINT fk_task_failures_worker FOREIGN KEY (worker_id) REFERENCES workers(id) ON DELETE CASCADE
);
INSERT INTO temp_task_failures SELECT created_at, task_id, worker_id FROM task_failures;
DROP TABLE task_failures;
ALTER TABLE temp_task_failures RENAME TO task_failures;

CREATE TABLE temp_worker_tag_membership (
  worker_tag_id integer NOT NULL,
  worker_id integer NOT NULL,
  PRIMARY KEY (worker_tag_id, worker_id),
  CONSTRAINT fk_worker_tag_membership_worker_tag FOREIGN KEY (worker_tag_id) REFERENCES worker_tags(id) ON DELETE CASCADE,
  CONSTRAINT fk_worker_tag_membership_worker FOREIGN KEY (worker_id) REFERENCES workers(id) ON DELETE CASCADE
);
INSERT INTO temp_worker_tag_membership SELECT worker_tag_id, worker_id FROM worker_tag_membership;
DROP TABLE worker_tag_membership;
ALTER TABLE temp_worker_tag_membership RENAME TO worker_tag_membership;

CREATE TABLE temp_worker_tags (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  updated_at datetime,
  uuid varchar(36) UNIQUE DEFAULT '' NOT NULL,
  name varchar(64) UNIQUE DEFAULT '' NOT NULL,
  description varchar(255) DEFAULT '' NOT NULL,
  PRIMARY KEY (id)
);
INSERT INTO temp_worker_tags SELECT
  id,
  created_at,
  updated_at,
  uuid,
  name,
  description
FROM worker_tags;
DROP TABLE worker_tags;
ALTER TABLE temp_worker_tags RENAME TO worker_tags;

CREATE TABLE temp_jobs (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  updated_at datetime,
  uuid varchar(36) UNIQUE DEFAULT '' NOT NULL,
  name varchar(64) DEFAULT '' NOT NULL,
  job_type varchar(32) DEFAULT '' NOT NULL,
  priority smallint DEFAULT 0 NOT NULL,
  status varchar(32) DEFAULT '' NOT NULL,
  activity varchar(255) DEFAULT '' NOT NULL,
  settings jsonb NOT NULL,
  metadata jsonb NOT NULL,
  delete_requested_at datetime,
  storage_shaman_checkout_id varchar(255) DEFAULT '' NOT NULL,
  worker_tag_id integer,
  PRIMARY KEY (id),
  CONSTRAINT fk_jobs_worker_tag FOREIGN KEY (worker_tag_id) REFERENCES worker_tags(id) ON DELETE SET NULL
);
INSERT INTO temp_jobs SELECT
  id,
  created_at,
  updated_at,
  uuid,
  name,
  job_type,
  priority,
  status,
  activity,
  settings,
  metadata,
  delete_requested_at,
  storage_shaman_checkout_id,
  worker_tag_id
FROM jobs;
DROP TABLE jobs;
ALTER TABLE temp_jobs RENAME TO jobs;

CREATE TABLE temp_workers (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  updated_at datetime,
  uuid varchar(36) UNIQUE DEFAULT '' NOT NULL,
  secret varchar(255) DEFAULT '' NOT NULL,
  name varchar(64) DEFAULT '' NOT NULL,
  address varchar(39) DEFAULT '' NOT NULL,
  platform varchar(16) DEFAULT '' NOT NULL,
  software varchar(32) DEFAULT '' NOT NULL,
  status varchar(16) DEFAULT '' NOT NULL,
  last_seen_at datetime,
  status_requested varchar(16) DEFAULT '' NOT NULL,
  lazy_status_request smallint DEFAULT false NOT NULL,
  supported_task_types varchar(255) DEFAULT '' NOT NULL,
  deleted_at datetime,
  can_restart smallint DEFAULT false NOT NULL,
  PRIMARY KEY (id)
);
UPDATE workers SET supported_task_types = '' where supported_task_types is NULL;
INSERT INTO temp_workers SELECT
  id,
  created_at,
  updated_at,
  uuid,
  secret,
  name,
  address,
  platform,
  software,
  status,
  last_seen_at,
  status_requested,
  lazy_status_request,
  supported_task_types,
  deleted_at,
  can_restart
FROM workers;
DROP TABLE workers;
ALTER TABLE temp_workers RENAME TO workers;

CREATE TABLE temp_job_blocks (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  job_id integer DEFAULT 0 NOT NULL,
  worker_id integer DEFAULT 0 NOT NULL,
  task_type text NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_job_blocks_job FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE,
  CONSTRAINT fk_job_blocks_worker FOREIGN KEY (worker_id) REFERENCES workers(id) ON DELETE CASCADE
);
INSERT INTO temp_job_blocks SELECT
  id,
  created_at,
  job_id,
  worker_id,
  task_type
FROM job_blocks;
DROP TABLE job_blocks;
ALTER TABLE temp_job_blocks RENAME TO job_blocks;

CREATE TABLE temp_sleep_schedules (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  updated_at datetime,
  worker_id integer UNIQUE DEFAULT 0 NOT NULL,
  is_active numeric DEFAULT false NOT NULL,
  days_of_week text DEFAULT '' NOT NULL,
  start_time text DEFAULT '' NOT NULL,
  end_time text DEFAULT '' NOT NULL,
  next_check datetime,
  PRIMARY KEY (id),
  CONSTRAINT fk_sleep_schedules_worker FOREIGN KEY (worker_id) REFERENCES workers(id) ON DELETE CASCADE
);
INSERT INTO temp_sleep_schedules SELECT
  id,
  created_at,
  updated_at,
  worker_id,
  is_active,
  days_of_week,
  start_time,
  end_time,
  next_check
FROM sleep_schedules;
DROP TABLE sleep_schedules;
ALTER TABLE temp_sleep_schedules RENAME TO sleep_schedules;

CREATE TABLE temp_tasks (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  updated_at datetime,
  uuid varchar(36) UNIQUE DEFAULT '' NOT NULL,
  name varchar(64) DEFAULT '' NOT NULL,
  type varchar(32) DEFAULT '' NOT NULL,
  job_id integer DEFAULT 0 NOT NULL,
  priority smallint DEFAULT 50 NOT NULL,
  status varchar(16) DEFAULT '' NOT NULL,
  worker_id integer,
  last_touched_at datetime,
  commands jsonb NOT NULL,
  activity varchar(255) DEFAULT '' NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_tasks_job FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE,
  CONSTRAINT fk_tasks_worker FOREIGN KEY (worker_id) REFERENCES workers(id) ON DELETE SET NULL
);
INSERT INTO temp_tasks SELECT
  id,
  created_at,
  updated_at,
  uuid,
  name,
  type,
  job_id,
  priority,
  status,
  worker_id,
  last_touched_at,
  commands,
  activity
FROM tasks;
DROP TABLE tasks;
ALTER TABLE temp_tasks RENAME TO tasks;

-- Recreate the indices on the new tables.
CREATE INDEX idx_worker_tags_uuid ON worker_tags(uuid);
CREATE INDEX idx_jobs_uuid ON jobs(uuid);
CREATE INDEX idx_workers_address ON workers(address);
CREATE INDEX idx_workers_last_seen_at ON workers(last_seen_at);
CREATE INDEX idx_workers_deleted_at ON workers(deleted_at);
CREATE INDEX idx_workers_uuid ON workers(uuid);
CREATE UNIQUE INDEX job_worker_tasktype ON job_blocks(job_id, worker_id, task_type);
CREATE INDEX idx_sleep_schedules_is_active ON sleep_schedules(is_active);
CREATE INDEX idx_sleep_schedules_worker_id ON sleep_schedules(worker_id);
CREATE INDEX idx_tasks_uuid ON tasks(uuid);
CREATE INDEX idx_tasks_last_touched_at ON tasks(last_touched_at);

-- +goose Down

CREATE TABLE `temp_last_rendereds` (
  `id` integer,
  `created_at` datetime,
  `updated_at` datetime,
  `job_id` integer DEFAULT 0,
  PRIMARY KEY (`id`),
  CONSTRAINT `fk_last_rendereds_job` FOREIGN KEY (`job_id`) REFERENCES `jobs`(`id`) ON DELETE CASCADE
);
INSERT INTO temp_last_rendereds SELECT
  id,
  created_at,
  updated_at,
  job_id
FROM last_rendereds;
DROP TABLE last_rendereds;
ALTER TABLE temp_last_rendereds RENAME TO `last_rendereds`;

CREATE TABLE `temp_task_dependencies` (
  `task_id` integer,
  `dependency_id` integer,
  PRIMARY KEY (`task_id`, `dependency_id`),
  CONSTRAINT `fk_task_dependencies_task` FOREIGN KEY (`task_id`) REFERENCES `tasks`(`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_task_dependencies_dependencies` FOREIGN KEY (`dependency_id`) REFERENCES `tasks`(`id`) ON DELETE CASCADE
);
INSERT INTO temp_task_dependencies SELECT task_id, dependency_id FROM task_dependencies;
DROP TABLE task_dependencies;
ALTER TABLE temp_task_dependencies RENAME TO `task_dependencies`;

CREATE TABLE `temp_task_failures` (
  `created_at` datetime,
  `task_id` integer,
  `worker_id` integer,
  PRIMARY KEY (`task_id`, `worker_id`),
  CONSTRAINT `fk_task_failures_task` FOREIGN KEY (`task_id`) REFERENCES `tasks`(`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_task_failures_worker` FOREIGN KEY (`worker_id`) REFERENCES `workers`(`id`) ON DELETE CASCADE
);
INSERT INTO temp_task_failures SELECT created_at, task_id, worker_id FROM task_failures;
DROP TABLE task_failures;
ALTER TABLE temp_task_failures RENAME TO `task_failures`;

CREATE TABLE `temp_worker_tag_membership` (
  `worker_tag_id` integer,
  `worker_id` integer,
  PRIMARY KEY (`worker_tag_id`, `worker_id`),
  CONSTRAINT `fk_worker_tag_membership_worker_tag` FOREIGN KEY (`worker_tag_id`) REFERENCES `worker_tags`(`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_worker_tag_membership_worker` FOREIGN KEY (`worker_id`) REFERENCES `workers`(`id`) ON DELETE CASCADE
);
INSERT INTO temp_worker_tag_membership SELECT worker_tag_id, worker_id FROM worker_tag_membership;
DROP TABLE worker_tag_membership;
ALTER TABLE temp_worker_tag_membership RENAME TO `worker_tag_membership`;

CREATE TABLE "temp_worker_tags" (
  `id` integer,
  `created_at` datetime,
  `updated_at` datetime,
  `uuid` char(36) UNIQUE DEFAULT "",
  `name` varchar(64) UNIQUE DEFAULT "",
  `description` varchar(255) DEFAULT "",
  PRIMARY KEY (`id`)
);
INSERT INTO temp_worker_tags SELECT
  id,
  created_at,
  updated_at,
  uuid,
  name,
  description
FROM worker_tags;
DROP TABLE worker_tags;
ALTER TABLE temp_worker_tags RENAME TO `worker_tags`;

CREATE TABLE "temp_jobs" (
  `id` integer,
  `created_at` datetime,
  `updated_at` datetime,
  `uuid` char(36) UNIQUE DEFAULT "",
  `name` varchar(64) DEFAULT "",
  `job_type` varchar(32) DEFAULT "",
  `priority` smallint DEFAULT 0,
  `status` varchar(32) DEFAULT "",
  `activity` varchar(255) DEFAULT "",
  `settings` jsonb,
  `metadata` jsonb,
  `delete_requested_at` datetime,
  `storage_shaman_checkout_id` varchar(255) DEFAULT "",
  `worker_tag_id` integer,
  PRIMARY KEY(`id`),
  CONSTRAINT `fk_jobs_worker_tag` FOREIGN KEY(`worker_tag_id`) REFERENCES `worker_tags`(`id`) ON DELETE SET NULL
);
INSERT INTO temp_jobs SELECT
  id,
  created_at,
  updated_at,
  uuid,
  name,
  job_type,
  priority,
  status,
  activity,
  settings,
  metadata,
  delete_requested_at,
  storage_shaman_checkout_id,
  worker_tag_id
FROM jobs;
DROP TABLE jobs;
ALTER TABLE temp_jobs RENAME TO `jobs`;

CREATE TABLE "temp_workers" (
  `id` integer,
  `created_at` datetime,
  `updated_at` datetime,
  `deleted_at` datetime,
  `uuid` char(36) UNIQUE DEFAULT "",
  `secret` varchar(255) DEFAULT "",
  `name` varchar(64) DEFAULT "",
  `address` varchar(39) DEFAULT "",
  `platform` varchar(16) DEFAULT "",
  `software` varchar(32) DEFAULT "",
  `status` varchar(16) DEFAULT "",
  `last_seen_at` datetime,
  `status_requested` varchar(16) DEFAULT "",
  `lazy_status_request` smallint DEFAULT false,
  `supported_task_types` varchar(255) DEFAULT "",
  `can_restart` smallint DEFAULT false,
  PRIMARY KEY (`id`)
);
INSERT INTO temp_workers SELECT
  id,
  created_at,
  updated_at,
  deleted_at,
  uuid,
  secret,
  name,
  address,
  platform,
  software,
  status,
  last_seen_at,
  status_requested,
  lazy_status_request,
  supported_task_types,
  can_restart
FROM workers;
DROP TABLE workers;
ALTER TABLE temp_workers RENAME TO `workers`;

CREATE TABLE "temp_job_blocks" (
  `id` integer,
  `created_at` datetime,
  `job_id` integer DEFAULT 0,
  `worker_id` integer DEFAULT 0,
  `task_type` text,
  PRIMARY KEY (`id`),
  CONSTRAINT `fk_job_blocks_job` FOREIGN KEY (`job_id`) REFERENCES `jobs`(`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_job_blocks_worker` FOREIGN KEY (`worker_id`) REFERENCES `workers`(`id`) ON DELETE CASCADE
);
INSERT INTO temp_job_blocks SELECT
  id,
  created_at,
  job_id,
  worker_id,
  task_type
FROM job_blocks;
DROP TABLE job_blocks;
ALTER TABLE temp_job_blocks RENAME TO `job_blocks`;

CREATE TABLE "temp_sleep_schedules" (
  `id` integer,
  `created_at` datetime,
  `updated_at` datetime,
  `worker_id` integer UNIQUE DEFAULT 0,
  `is_active` numeric DEFAULT false,
  `days_of_week` text DEFAULT "",
  `start_time` text DEFAULT "",
  `end_time` text DEFAULT "",
  `next_check` datetime,
  PRIMARY KEY (`id`),
  CONSTRAINT `fk_sleep_schedules_worker` FOREIGN KEY (`worker_id`) REFERENCES `workers`(`id`) ON DELETE CASCADE
);
INSERT INTO temp_sleep_schedules SELECT
  id,
  created_at,
  updated_at,
  worker_id,
  is_active,
  days_of_week,
  start_time,
  end_time,
  next_check
FROM sleep_schedules;
DROP TABLE sleep_schedules;
ALTER TABLE temp_sleep_schedules RENAME TO `sleep_schedules`;

CREATE TABLE "temp_tasks" (
  `id` integer,
  `created_at` datetime,
  `updated_at` datetime,
  `uuid` char(36) UNIQUE DEFAULT "",
  `name` varchar(64) DEFAULT "",
  `type` varchar(32) DEFAULT "",
  `job_id` integer DEFAULT 0,
  `priority` smallint DEFAULT 50,
  `status` varchar(16) DEFAULT "",
  `worker_id` integer,
  `last_touched_at` datetime,
  `commands` jsonb,
  `activity` varchar(255) DEFAULT "",
  PRIMARY KEY (`id`),
  CONSTRAINT `fk_tasks_job` FOREIGN KEY (`job_id`) REFERENCES `jobs`(`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_tasks_worker` FOREIGN KEY (`worker_id`) REFERENCES `workers`(`id`) ON DELETE
  SET NULL
);
INSERT INTO temp_tasks SELECT
  id,
  created_at,
  updated_at,
  uuid,
  name,
  type,
  job_id,
  priority,
  status,
  worker_id,
  last_touched_at,
  commands,
  activity
FROM tasks;
DROP TABLE tasks;
ALTER TABLE temp_tasks RENAME TO `tasks`;

CREATE INDEX `idx_worker_tags_uuid` ON `worker_tags`(`uuid`);
CREATE INDEX `idx_jobs_uuid` ON `jobs`(`uuid`);
CREATE INDEX `idx_workers_address` ON `workers`(`address`);
CREATE INDEX `idx_workers_last_seen_at` ON `workers`(`last_seen_at`);
CREATE INDEX `idx_workers_deleted_at` ON `workers`(`deleted_at`);
CREATE INDEX `idx_workers_uuid` ON `workers`(`uuid`);
CREATE UNIQUE INDEX `job_worker_tasktype` ON `job_blocks`(`job_id`, `worker_id`, `task_type`);
CREATE INDEX `idx_sleep_schedules_is_active` ON `sleep_schedules`(`is_active`);
CREATE INDEX `idx_sleep_schedules_worker_id` ON `sleep_schedules`(`worker_id`);
CREATE INDEX `idx_tasks_uuid` ON `tasks`(`uuid`);
CREATE INDEX `idx_tasks_last_touched_at` ON `tasks`(`last_touched_at`);
