
-- name: CreateWorker :one
INSERT INTO workers (
  created_at,
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
) values (
  @created_at,
  @uuid,
  @secret,
  @name,
  @address,
  @platform,
  @software,
  @status,
  @last_seen_at,
  @status_requested,
  @lazy_status_request,
  @supported_task_types,
  @deleted_at,
  @can_restart
)
RETURNING id;

-- name: AddWorkerTagMembership :exec
INSERT INTO worker_tag_membership (worker_tag_id, worker_id)
VALUES (@worker_tag_id, @worker_id);

-- name: FetchWorkers :many
SELECT sqlc.embed(workers) FROM workers
WHERE deleted_at IS NULL;

-- name: FetchWorker :one
-- FetchWorker only returns the worker if it wasn't soft-deleted.
SELECT * FROM workers WHERE workers.uuid = @uuid and deleted_at is NULL;

-- name: FetchWorkerUnconditional :one
-- FetchWorkerUnconditional ignores soft-deletion status and just returns the worker.
SELECT * FROM workers WHERE workers.uuid = @uuid;

-- name: FetchWorkerUnconditionalByID :one
-- FetchWorkerUnconditional ignores soft-deletion status and just returns the worker.
SELECT * FROM workers WHERE workers.id = @worker_id;

-- name: FetchWorkerTags :many
SELECT worker_tags.*
FROM worker_tags
LEFT JOIN worker_tag_membership m ON (m.worker_tag_id = worker_tags.id)
LEFT JOIN workers on (m.worker_id = workers.id)
WHERE workers.uuid = @uuid;

-- name: FetchWorkerTagByUUID :one
SELECT sqlc.embed(worker_tags)
FROM worker_tags
WHERE worker_tags.uuid = @uuid;

-- name: SoftDeleteWorker :execrows
UPDATE workers SET deleted_at=@deleted_at
WHERE uuid=@uuid;

-- name: SaveWorkerStatus :exec
UPDATE workers SET
  updated_at=@updated_at,
  status=@status,
  status_requested=@status_requested,
  lazy_status_request=@lazy_status_request
WHERE id=@id;

-- name: SaveWorker :exec
UPDATE workers SET
  updated_at=@updated_at,
  uuid=@uuid,
  secret=@secret,
  name=@name,
  address=@address,
  platform=@platform,
  software=@software,
  status=@status,
  last_seen_at=@last_seen_at,
  status_requested=@status_requested,
  lazy_status_request=@lazy_status_request,
  supported_task_types=@supported_task_types,
  can_restart=@can_restart
WHERE id=@id;

-- name: WorkerSeen :exec
UPDATE workers SET
  updated_at=@updated_at,
  last_seen_at=@last_seen_at
WHERE id=@id;

-- name: SummarizeWorkerStatuses :many
SELECT status, count(id) as status_count FROM workers
WHERE deleted_at is NULL
GROUP BY status;
