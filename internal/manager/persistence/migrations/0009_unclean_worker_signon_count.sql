-- Add counter for unclean sign-on event detections on the worker

-- +goose Up
ALTER TABLE workers ADD COLUMN unclean_signon_count integer DEFAULT 0 NOT NULL;

-- +goose Down
ALTER TABLE workers DROP COLUMN unclean_signon_count;