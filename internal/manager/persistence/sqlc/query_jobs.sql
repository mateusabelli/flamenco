
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

-- name: FetchJob :one
SELECT * FROM jobs
WHERE uuid = ? LIMIT 1;

-- name: DeleteJob :exec
DELETE FROM jobs WHERE uuid = ?;

-- name: RequestJobDeletion :exec
UPDATE jobs SET
  updated_at = @now,
  delete_requested_at = @now
WHERE id = sqlc.arg('job_id');

-- name: FetchJobUUIDsUpdatedBefore :many
SELECT uuid FROM jobs WHERE updated_at <= @updated_at_max;

-- name: RequestMassJobDeletion :exec
UPDATE jobs SET
  updated_at = @now,
  delete_requested_at = @now
WHERE uuid in (sqlc.slice('uuids'));

-- name: FetchJobsDeletionRequested :many
SELECT uuid FROM jobs
  WHERE delete_requested_at is not NULL
  ORDER BY delete_requested_at;

-- name: FetchJobsInStatus :many
SELECT * FROM jobs WHERE status IN (sqlc.slice('statuses'));

-- name: SaveJobStatus :exec
UPDATE jobs SET updated_at=@now, status=@status, activity=@activity WHERE id=@id;

-- name: SaveJobPriority :exec
UPDATE jobs SET updated_at=@now, priority=@priority WHERE id=@id;

-- name: SaveJobStorageInfo :exec
UPDATE jobs SET storage_shaman_checkout_id=@storage_shaman_checkout_id WHERE id=@id;
