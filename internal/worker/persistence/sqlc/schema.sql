CREATE TABLE IF NOT EXISTS task_updates (
  id integer NOT NULL,
  created_at datetime NOT NULL,
  task_id varchar(36) DEFAULT '' NOT NULL,
  payload BLOB NOT NULL,
  PRIMARY KEY (id)
);
