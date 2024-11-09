-- Add sequence numbers to tasks, to indicate their creation order within their job.
--
-- +goose Up

CREATE TABLE temp_tasks (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  updated_at datetime,
  uuid varchar(36) UNIQUE DEFAULT '' NOT NULL,
  name varchar(64) DEFAULT '' NOT NULL,
  type varchar(32) DEFAULT '' NOT NULL,
  job_id integer DEFAULT 0 NOT NULL,
  index_in_job integer DEFAULT 0 NOT NULL,
  priority smallint DEFAULT 50 NOT NULL,
  status varchar(16) DEFAULT '' NOT NULL,
  worker_id integer,
  last_touched_at datetime,
  commands jsonb NOT NULL,
  activity varchar(255) DEFAULT '' NOT NULL,
  PRIMARY KEY (id),
  UNIQUE(job_id, index_in_job),
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
  ROW_NUMBER() OVER (
    PARTITION BY job_id
    ORDER BY rowid
  ),
  priority,
  status,
  worker_id,
  last_touched_at,
  commands,
  activity
FROM tasks;
DROP TABLE tasks;
ALTER TABLE temp_tasks RENAME TO tasks;

-- +goose Down
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
