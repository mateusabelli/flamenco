
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

-- name: WorkerAddTagMembership :exec
INSERT INTO worker_tag_membership (worker_tag_id, worker_id)
VALUES (@worker_tag_id, @worker_id);

-- name: WorkerRemoveTagMemberships :exec
DELETE
FROM worker_tag_membership
WHERE worker_id=@worker_id;

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

-- name: FetchTagsOfWorker :many
SELECT worker_tags.*
FROM worker_tags
LEFT JOIN worker_tag_membership m ON (m.worker_tag_id = worker_tags.id)
LEFT JOIN workers on (m.worker_id = workers.id)
WHERE workers.uuid = @uuid;

-- name: FetchWorkerTags :many
SELECT *
FROM worker_tags;

-- name: FetchWorkerTagByUUID :one
SELECT *
FROM worker_tags
WHERE worker_tags.uuid = @uuid;

-- name: FetchWorkerTagsByUUIDs :many
SELECT *
FROM worker_tags
WHERE uuid in (sqlc.slice('uuids'));

-- name: FetchWorkerTagByID :one
SELECT *
FROM worker_tags
WHERE id=@worker_tag_id;

-- name: SaveWorkerTag :exec
UPDATE worker_tags
SET
  updated_at=@updated_at,
  uuid=@uuid,
  name=@name,
  description=@description
WHERE id=@worker_tag_id;

-- name: DeleteWorkerTag :execrows
DELETE FROM worker_tags
WHERE uuid=@uuid;

-- name: CreateWorkerTag :execlastid
INSERT INTO worker_tags (
  created_at,
  uuid,
  name,
  description
) VALUES (
  @created_at,
  @uuid,
  @name,
  @description
);

-- name: CountWorkerTags :one
SELECT count(id) as count FROM worker_tags;

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
