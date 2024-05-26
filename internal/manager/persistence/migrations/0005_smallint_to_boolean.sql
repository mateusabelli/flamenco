-- Some booleans were modeled as `smallint`. These are turned into `boolean` instead.
--
-- +goose Up
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
  lazy_status_request boolean DEFAULT false NOT NULL,
  supported_task_types varchar(255) DEFAULT '' NOT NULL,
  deleted_at datetime,
  can_restart boolean DEFAULT false NOT NULL,
  PRIMARY KEY (id)
);
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


-- +goose Down
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
