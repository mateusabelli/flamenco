
-- Jobs / Tasks queries
--

-- name: CreateJob :exec
INSERT INTO jobs (
  created_at,
  uuid,
  name,
  job_type,
  priority,
  status,
  activity,
  settings,
  metadata,
  storage_shaman_checkout_id
)
VALUES ( ?, ?, ?, ?, ?, ?, ?, ?, ?, ? );

-- name: DeleteJob :exec
DELETE FROM jobs WHERE uuid = ?;

-- name: RequestJobDeletion :exec
UPDATE jobs SET
  updated_at = @now,
  delete_requested_at = @now
WHERE id = sqlc.arg('job_id');

-- name: FetchTask :one
SELECT * FROM tasks
WHERE uuid = ? LIMIT 1;

-- name: FetchJob :one
SELECT * FROM jobs
WHERE uuid = ? LIMIT 1;
