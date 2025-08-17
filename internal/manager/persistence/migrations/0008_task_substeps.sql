-- Add task substeps

-- +goose Up
ALTER TABLE tasks ADD COLUMN steps_completed integer DEFAULT 0 NOT NULL;
ALTER TABLE tasks ADD COLUMN steps_total integer DEFAULT 0 NOT NULL;
ALTER TABLE jobs ADD COLUMN steps_completed integer DEFAULT 0 NOT NULL;
ALTER TABLE jobs ADD COLUMN steps_total integer DEFAULT 0 NOT NULL;

-- +goose Down
ALTER TABLE tasks DROP COLUMN steps_completed;
ALTER TABLE tasks DROP COLUMN steps_total;
ALTER TABLE jobs DROP COLUMN steps_completed;
ALTER TABLE jobs DROP COLUMN steps_total;
