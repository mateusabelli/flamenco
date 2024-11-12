
-- name: FetchAssignedAndRunnableTaskOfWorker :one
-- Fetch a task that's assigned to this worker, and is in a runnable state.
SELECT sqlc.embed(tasks), jobs.uuid as jobuuid, jobs.priority as job_priority, jobs.job_type as job_type
FROM tasks
  INNER JOIN jobs ON tasks.job_id = jobs.id
WHERE tasks.status=@active_task_status
  AND tasks.worker_id=@worker_id
  AND jobs.status IN (sqlc.slice('active_job_statuses'))
LIMIT 1;


-- name: FindRunnableTask :one
-- Find a task to be run by a worker. This is the core of the task scheduler.
--
-- Note that this query doesn't check for the assigned worker. Tasks that have a
-- 'schedulable' status might have been assigned to a worker, representing the
-- last worker to touch it -- it's not meant to indicate "ownership" of the
-- task.
--
-- The order in the WHERE clause is important, slices should come last. See
-- https://github.com/sqlc-dev/sqlc/issues/2452 for more info.
SELECT sqlc.embed(tasks), jobs.uuid as jobuuid, jobs.priority as job_priority, jobs.job_type as job_type
FROM tasks
  INNER JOIN jobs ON tasks.job_id = jobs.id
  LEFT JOIN task_failures TF ON tasks.id = TF.task_id AND TF.worker_id=@worker_id
WHERE TF.worker_id IS NULL -- Not failed by this worker before.
  AND tasks.id NOT IN (
    -- Find all tasks IDs that have incomplete dependencies. These are not runnable.
    SELECT tasks_incomplete.id
    FROM tasks AS tasks_incomplete
      INNER JOIN task_dependencies td ON tasks_incomplete.id = td.task_id
      INNER JOIN tasks dep ON dep.id = td.dependency_id
    WHERE dep.status != @task_status_completed
  )
  AND tasks.type NOT IN (
    SELECT task_type
    FROM job_blocks
    WHERE job_blocks.worker_id = @worker_id
      AND job_blocks.job_id = jobs.id
  )
  AND (
    jobs.worker_tag_id IS NULL
    OR jobs.worker_tag_id IN (sqlc.slice('worker_tags')))
  AND tasks.status IN (sqlc.slice('schedulable_task_statuses'))
  AND jobs.status IN (sqlc.slice('schedulable_job_statuses'))
  AND tasks.type IN (sqlc.slice('supported_task_types'))
ORDER BY jobs.priority DESC, tasks.priority DESC;

-- name: FetchWorkerTask :one
-- Find the currently-active task assigned to a Worker. If not found, find the last task this Worker worked on.
SELECT
  sqlc.embed(tasks),
  jobs.uuid as jobuuid,
  CAST(tasks.status = @task_status_active AND jobs.status = @job_status_active AS BOOLEAN) as is_active
FROM tasks
  INNER JOIN jobs ON tasks.job_id = jobs.id
WHERE
  tasks.worker_id = @worker_id
ORDER BY
  is_active DESC,
  tasks.updated_at DESC
LIMIT 1;

-- name: AssignTaskToWorker :exec
UPDATE tasks
SET worker_id=@worker_id, last_touched_at=@now, updated_at=@now
WHERE tasks.id=@task_id;
