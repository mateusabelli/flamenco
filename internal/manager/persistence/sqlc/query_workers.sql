
-- Worker queries
--

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
