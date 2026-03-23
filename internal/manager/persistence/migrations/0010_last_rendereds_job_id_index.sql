-- Add index to last_rendereds(job_id).

-- +goose Up
CREATE UNIQUE INDEX IF NOT EXISTS last_rendereds_job_id ON last_rendereds(job_id);

-- +goose Down
DROP INDEX IF EXISTS last_rendereds_job_id;
