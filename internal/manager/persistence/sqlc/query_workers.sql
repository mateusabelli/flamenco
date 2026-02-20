
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
  can_restart,
  unclean_signon_count
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
  @can_restart,
  @unclean_signon_count
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
SELECT * FROM workers
WHERE deleted_at IS NULL;

-- name: FetchWorker :one
-- FetchWorker only returns the worker if it wasn't soft-deleted.
SELECT * FROM workers WHERE workers.uuid = @uuid and deleted_at is NULL;

-- name: FetchWorkerByID :one
-- FetchWorkerByID only returns the worker if it wasn't soft-deleted.
SELECT * FROM workers WHERE workers.id = @worker_id and deleted_at is NULL;

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

-- name: FetchWorkerTagByName :one
SELECT *
FROM worker_tags
WHERE worker_tags.name = @name;

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

-- name: FetchTimedOutWorkers :many
SELECT *
FROM workers
WHERE
    last_seen_at <= @last_seen_before
AND deleted_at IS NULL
AND status NOT IN (sqlc.slice('worker_statuses_no_timeout'));

-- name: FetchWorkerSleepSchedule :one
SELECT sleep_schedules.*
FROM sleep_schedules
INNER JOIN workers on workers.id = sleep_schedules.worker_id
WHERE workers.uuid = @workerUUID;

-- name: SetWorkerSleepSchedule :execlastid
-- Note that the use of ?2 and ?3 in the SQL is not desirable, and should be
-- replaced with @updated_at and @job_id as soon as sqlc issue #3334 is fixed.
-- See https://github.com/sqlc-dev/sqlc/issues/3334 for more info.
INSERT INTO sleep_schedules (
  created_at,
  updated_at,
  worker_id,
  is_active,
  days_of_week,
  start_time,
  end_time,
  next_check
) VALUES (
  @created_at,
  @updated_at,
  @worker_id,
  @is_active,
  @days_of_week,
  @start_time,
  @end_time,
  @next_check
)
ON CONFLICT DO UPDATE
  SET updated_at=?2, is_active=?4, days_of_week=?5, start_time=?6, end_time=?7, next_check=?8
  WHERE worker_id=?3;

-- name: SetWorkerSleepScheduleNextCheck :execrows
UPDATE sleep_schedules
SET next_check=@next_check
WHERE ID=@schedule_id;


-- name: FetchSleepSchedulesToCheck :many
SELECT sqlc.embed(sleep_schedules), workers.uuid as workeruuid, workers.name as worker_name
FROM sleep_schedules
LEFT JOIN workers ON workers.id = sleep_schedules.worker_id
WHERE is_active
AND (next_check <= @next_check OR next_check IS NULL OR next_check = '');

-- name: Test_CreateWorkerSleepSchedule :execlastid
INSERT INTO sleep_schedules (
  created_at,
  worker_id,
  is_active,
  days_of_week,
  start_time,
  end_time,
  next_check
) VALUES (
  @created_at,
  @worker_id,
  @is_active,
  @days_of_week,
  @start_time,
  @end_time,
  @next_check
);

-- name: IncrementUncleanSignOnCount :exec
UPDATE workers
SET unclean_signon_count = unclean_signon_count + 1
WHERE uuid = @uuid;

-- name: ResetUncleanSignOnCount :exec
UPDATE workers
SET unclean_signon_count = 0
WHERE uuid = @uuid;