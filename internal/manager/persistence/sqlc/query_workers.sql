
-- Worker queries
--

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

-- name: FetchWorker :one
-- FetchWorker only returns the worker if it wasn't soft-deleted.
SELECT * FROM workers WHERE workers.uuid = @uuid and deleted_at is NULL;

-- name: FetchWorkerUnconditional :one
-- FetchWorkerUnconditional ignores soft-deletion status and just returns the worker.
SELECT * FROM workers WHERE workers.uuid = @uuid;

-- name: FetchWorkerTags :many
SELECT worker_tags.*
FROM worker_tags
LEFT JOIN worker_tag_membership m ON (m.worker_tag_id = worker_tags.id)
LEFT JOIN workers on (m.worker_id = workers.id)
WHERE workers.uuid = @uuid;
