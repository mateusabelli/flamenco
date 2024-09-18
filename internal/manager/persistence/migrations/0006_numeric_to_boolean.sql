-- Some booleans were modeled as `numeric`. These are turned into `boolean` instead.
--
-- +goose Up
CREATE TABLE temp_sleep_schedules (
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

-- +goose Down
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
