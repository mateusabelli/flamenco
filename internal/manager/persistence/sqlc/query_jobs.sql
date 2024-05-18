
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
-- Fetch a job by its UUID.
SELECT * FROM jobs
WHERE uuid = ? LIMIT 1;

-- name: FetchJobByID :one
-- Fetch a job by its numerical ID.
SELECT * FROM jobs
WHERE id = ? LIMIT 1;

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

-- name: FetchTask :one
SELECT sqlc.embed(tasks), jobs.UUID as jobUUID, workers.UUID as workerUUID
FROM tasks
LEFT JOIN jobs ON (tasks.job_id = jobs.id)
LEFT JOIN workers ON (tasks.worker_id = workers.id)
WHERE tasks.uuid = @uuid;

-- name: FetchTaskJobUUID :one
SELECT jobs.UUID as jobUUID
FROM tasks
LEFT JOIN jobs ON (tasks.job_id = jobs.id)
WHERE tasks.uuid = @uuid;

-- name: UpdateTask :exec
-- Update a Task, except its id, created_at, uuid, or job_id fields.
UPDATE tasks SET
  updated_at = @updated_at,
  name = @name,
  type = @type,
  priority = @priority,
  status = @status,
  worker_id = @worker_id,
  last_touched_at = @last_touched_at,
  commands = @commands,
  activity = @activity
WHERE id=@id;

-- name: UpdateTaskStatus :exec
UPDATE tasks SET
  updated_at = @updated_at,
  status = @status
WHERE id=@id;

-- name: UpdateTaskActivity :exec
UPDATE tasks SET
  updated_at = @updated_at,
  activity = @activity
WHERE id=@id;
