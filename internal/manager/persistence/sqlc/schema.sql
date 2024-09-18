CREATE TABLE job_blocks (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  job_id integer DEFAULT 0 NOT NULL,
  worker_id integer DEFAULT 0 NOT NULL,
  task_type text NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_job_blocks_job FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE,
  CONSTRAINT fk_job_blocks_worker FOREIGN KEY (worker_id) REFERENCES workers(id) ON DELETE CASCADE
);
CREATE TABLE jobs (
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
CREATE TABLE last_rendereds (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  updated_at datetime,
  job_id integer DEFAULT 0 NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_last_rendereds_job FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);
CREATE TABLE sleep_schedules (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  updated_at datetime,
  worker_id integer UNIQUE DEFAULT 0 NOT NULL,
  is_active boolean DEFAULT false NOT NULL,
  days_of_week text DEFAULT '' NOT NULL,
  start_time text DEFAULT '' NOT NULL,
  end_time text DEFAULT '' NOT NULL,
  next_check datetime,
  PRIMARY KEY (id),
  CONSTRAINT fk_sleep_schedules_worker FOREIGN KEY (worker_id) REFERENCES workers(id) ON DELETE CASCADE
);
CREATE TABLE task_dependencies (
  task_id integer NOT NULL,
  dependency_id integer NOT NULL,
  PRIMARY KEY (task_id, dependency_id),
  CONSTRAINT fk_task_dependencies_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
  CONSTRAINT fk_task_dependencies_dependencies FOREIGN KEY (dependency_id) REFERENCES tasks(id) ON DELETE CASCADE
);
CREATE TABLE task_failures (
  created_at datetime NOT NULL,
  task_id integer NOT NULL,
  worker_id integer NOT NULL,
  PRIMARY KEY (task_id, worker_id),
  CONSTRAINT fk_task_failures_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
  CONSTRAINT fk_task_failures_worker FOREIGN KEY (worker_id) REFERENCES workers(id) ON DELETE CASCADE
);
CREATE TABLE tasks (
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
CREATE TABLE worker_tag_membership (
  worker_tag_id integer NOT NULL,
  worker_id integer NOT NULL,
  PRIMARY KEY (worker_tag_id, worker_id),
  CONSTRAINT fk_worker_tag_membership_worker_tag FOREIGN KEY (worker_tag_id) REFERENCES worker_tags(id) ON DELETE CASCADE,
  CONSTRAINT fk_worker_tag_membership_worker FOREIGN KEY (worker_id) REFERENCES workers(id) ON DELETE CASCADE
);
CREATE TABLE worker_tags (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  updated_at datetime,
  uuid varchar(36) UNIQUE DEFAULT '' NOT NULL,
  name varchar(64) UNIQUE DEFAULT '' NOT NULL,
  description varchar(255) DEFAULT '' NOT NULL,
  PRIMARY KEY (id)
);
CREATE TABLE workers (
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
  lazy_status_request boolean DEFAULT false NOT NULL,
  supported_task_types varchar(255) DEFAULT '' NOT NULL,
  deleted_at datetime,
  can_restart boolean DEFAULT false NOT NULL,
  PRIMARY KEY (id)
);
CREATE INDEX idx_jobs_uuid ON jobs(uuid);
CREATE INDEX idx_sleep_schedules_is_active ON sleep_schedules(is_active);
CREATE INDEX idx_sleep_schedules_worker_id ON sleep_schedules(worker_id);
CREATE INDEX idx_tasks_last_touched_at ON tasks(last_touched_at);
CREATE INDEX idx_tasks_uuid ON tasks(uuid);
CREATE INDEX idx_worker_tags_uuid ON worker_tags(uuid);
CREATE INDEX idx_workers_address ON workers(address);
CREATE INDEX idx_workers_deleted_at ON workers(deleted_at);
CREATE INDEX idx_workers_last_seen_at ON workers(last_seen_at);
CREATE INDEX idx_workers_uuid ON workers(uuid);
CREATE UNIQUE INDEX job_worker_tasktype ON job_blocks(job_id, worker_id, task_type);
