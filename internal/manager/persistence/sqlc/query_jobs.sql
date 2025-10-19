-- name: CreateJob :execlastid
INSERT INTO jobs (
  created_at,
  updated_at,
  uuid,
  name,
  job_type,
  priority,
  status,
  activity,
  settings,
  metadata,
  storage_shaman_checkout_id,
  worker_tag_id,
  steps_total
)
VALUES (
  @created_at,
  @created_at,
  @uuid,
  @name,
  @job_type,
  @priority,
  @status,
  @activity,
  @settings,
  @metadata,
  @storage_shaman_checkout_id,
  @worker_tag_id,
  @steps_total
);

-- name: CreateTask :execlastid
INSERT INTO tasks (
  created_at,
  updated_at,
  uuid,
  name,
  type,
  job_id,
  index_in_job,
  priority,
  status,
  commands,
  steps_total
) VALUES (
  @created_at,
  @created_at,
  @uuid,
  @name,
  @type,
  @job_id,
  @index_in_job,
  @priority,
  @status,
  @commands,
  @steps_total
);

-- name: StoreTaskDependency :exec
INSERT INTO task_dependencies (task_id, dependency_id) VALUES (@task_id, @dependency_id);

-- name: FetchJob :one
-- Fetch a job by its UUID.
SELECT * FROM jobs
WHERE uuid = ? LIMIT 1;

-- name: FetchJobByID :one
-- Fetch a job by its numerical ID.
SELECT * FROM jobs
WHERE id = ? LIMIT 1;

-- name: FetchJobs :many
-- Fetch all jobs in the database.
SELECT * fRoM jobs;

-- name: FetchJobShamanCheckoutID :one
SELECT storage_shaman_checkout_id FROM jobs WHERE uuid=@uuid;

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

-- name: SaveJobWorkerTag :exec
UPDATE jobs SET updated_at=@now, worker_tag_id=@worker_tag_id WHERE id=@id;

-- name: SaveJobStorageInfo :exec
UPDATE jobs SET storage_shaman_checkout_id=@storage_shaman_checkout_id WHERE id=@id;

-- name: FetchTask :one
SELECT sqlc.embed(tasks), jobs.UUID as jobUUID, workers.UUID as workerUUID
FROM tasks
LEFT JOIN jobs ON (tasks.job_id = jobs.id)
LEFT JOIN workers ON (tasks.worker_id = workers.id)
WHERE tasks.uuid = @uuid;

-- name: FetchTasksOfWorkerInStatus :many
SELECT sqlc.embed(tasks), jobs.uuid as jobuuid
FROM tasks
INNER JOIN jobs ON (tasks.job_id = jobs.id)
WHERE tasks.worker_id = @worker_id
  AND tasks.status = @task_status;

-- name: FetchTasksOfWorkerInStatusOfJob :many
SELECT sqlc.embed(tasks)
FROM tasks
LEFT JOIN jobs ON (tasks.job_id = jobs.id)
WHERE tasks.worker_id = @worker_id
  AND tasks.status = @task_status
  AND jobs.uuid = @jobuuid;

-- name: FetchTasksOfJob :many
SELECT sqlc.embed(tasks), workers.UUID as workerUUID
FROM tasks
LEFT JOIN workers ON (tasks.worker_id = workers.id)
WHERE tasks.job_id = @job_id;

-- name: FetchTasksOfJobInStatus :many
SELECT sqlc.embed(tasks), workers.UUID as workerUUID
FROM tasks
LEFT JOIN workers ON (tasks.worker_id = workers.id)
WHERE tasks.job_id = @job_id
  AND tasks.status in (sqlc.slice('task_status'));

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
  activity = @activity,
  steps_total = @steps_total,
  steps_completed = @steps_completed
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

-- name: UpdateJobStepsCompleted :exec
-- Sum the steps_completed of all this job's tasks, and store it on the job.
-- This could use 'UPDATE FROM' syntax, once https://github.com/sqlc-dev/sqlc/issues/3132 is fixed.
UPDATE jobs
SET
  updated_at = @updated_at,
  steps_completed = (
    SELECT COALESCE(SUM(tasks.steps_completed), 0)
    FROM tasks
    WHERE tasks.job_id = @id
  ),
  steps_total = (
    SELECT COALESCE(SUM(tasks.steps_total), 0)
    FROM tasks
    WHERE tasks.job_id = @id
  )
WHERE id = @id;

-- name: UpdateTaskStepsCompleted :exec
UPDATE tasks SET
  updated_at = @updated_at,
  steps_completed = @steps_completed
WHERE id=@id;

-- name: UpdateJobsTaskStatusesConditional :exec
UPDATE tasks SET
  updated_at = @updated_at,
  status = @status,
  activity = @activity
WHERE job_id = @job_id AND status in (sqlc.slice('statuses_to_update'));

-- name: UpdateJobsTaskStepCountsComplete :exec
-- Set the job's tasks with the given status to their total step count.
UPDATE tasks SET
  updated_at = @updated_at,
  steps_completed = steps_total
WHERE job_id = @job_id AND status in (sqlc.slice('statuses_to_update'));

-- name: UpdateJobsTaskStepCountsZero :exec
-- Set the job's tasks with the given status to zero completed step count.
UPDATE tasks SET
  updated_at = @updated_at,
  steps_completed = 0
WHERE job_id = @job_id AND status in (sqlc.slice('statuses_to_update'));

-- name: UpdateJobsTaskStatuses :exec
UPDATE tasks SET
  updated_at = @updated_at,
  status = @status,
  activity = @activity
WHERE job_id = @job_id;

-- name: TaskAssignToWorker :exec
UPDATE tasks SET
  updated_at = @updated_at,
  worker_id = @worker_id
WHERE id=@id;

-- name: TaskTouchedByWorker :exec
UPDATE tasks SET
  updated_at = @updated_at,
  last_touched_at = @last_touched_at
WHERE uuid=@uuid;

-- name: JobCountTasksInStatus :one
-- Fetch number of tasks in the given status, of the given job.
SELECT count(*) as num_tasks FROM tasks
WHERE job_id = @job_id AND status = @task_status;

-- name: JobCountTaskStatuses :many
-- Fetch (status, num tasks in that status) rows for the given job.
SELECT status, count(*) as num_tasks FROM tasks
WHERE job_id = @job_id
GROUP BY status;

-- name: AddWorkerToTaskFailedList :exec
INSERT INTO task_failures (created_at, task_id, worker_id)
VALUES (@created_at, @task_id, @worker_id)
ON CONFLICT DO NOTHING;

-- name: CountWorkersFailingTask :one
-- Count how many workers have failed a given task.
SELECT count(*) as num_failed FROM task_failures
WHERE task_id=@task_id;

-- name: ClearFailureListOfTask :exec
DELETE FROM task_failures WHERE task_id=@task_id;

-- name: ClearFailureListOfJob :exec
-- SQLite doesn't support JOIN in DELETE queries, so use a sub-query instead.
DELETE FROM task_failures
WHERE task_id in (SELECT id FROM tasks WHERE job_id=@job_id);

-- name: FetchTaskFailureList :many
SELECT sqlc.embed(workers) FROM workers
INNER JOIN task_failures TF on TF.worker_id=workers.id
WHERE TF.task_id=@task_id;

-- name: FetchJobIDFromUUID :one
-- Fetch the job's database ID by its UUID.
--
-- This query is here to keep the SetLastRendered query below simpler,
-- mostly because that query is already hitting a limitation of sqlc.
SELECT id FROM jobs WHERE uuid=@jobuuid;

-- name: SetLastRendered :exec
-- Set the 'last rendered' job info.
--
-- Note that the use of ?2 and ?3 in the SQL is not desirable, and should be
-- replaced with @updated_at and @job_id as soon as sqlc issue #3334 is fixed.
-- See https://github.com/sqlc-dev/sqlc/issues/3334 for more info.
INSERT INTO last_rendereds (id, created_at, updated_at, job_id)
VALUES (1, @created_at, @updated_at, @job_id)
ON CONFLICT DO UPDATE
  SET updated_at=?2, job_id=?3
  WHERE id=1;

-- name: GetLastRenderedJobUUID :one
SELECT uuid FROM jobs
INNER JOIN last_rendereds LR ON jobs.id = LR.job_id;

-- name: AddWorkerToJobBlocklist :exec
-- Add a worker to a job's blocklist.
INSERT INTO job_blocks (created_at, job_id, worker_id, task_type)
VALUES (@created_at, @job_id, @worker_id, @task_type)
ON CONFLICT DO NOTHING;

-- name: FetchJobBlocklist :many
-- Fetch the blocklist of a specific job.
SELECT job_blocks.id, job_blocks.task_type, workers.uuid as workeruuid, workers.name as worker_name
FROM job_blocks
INNER JOIN jobs ON jobs.id = job_blocks.job_id
INNER JOIN workers on workers.id = job_blocks.worker_id
WHERE jobs.uuid = @jobuuid
ORDER BY workers.name;

-- name: ClearJobBlocklist :exec
DELETE FROM job_blocks
WHERE job_id in (SELECT jobs.id FROM jobs WHERE jobs.uuid=@jobuuid);

-- name: RemoveFromJobBlocklist :exec
DELETE FROM job_blocks
WHERE
    job_blocks.job_id in (SELECT jobs.id FROM jobs WHERE jobs.uuid=@jobuuid)
AND job_blocks.worker_id in (SELECT workers.id FROM workers WHERE workers.uuid=@workeruuid)
AND job_blocks.task_type = @task_type;

-- name: Test_FetchJobBlocklist :many
-- Fetch all job block list entries. Used only in unit tests.
SELECT * FROM job_blocks;

-- name: WorkersLeftToRun :many
SELECT workers.uuid FROM workers
WHERE id NOT IN (
  SELECT blocked_workers.id
  FROM workers AS blocked_workers
  INNER JOIN job_blocks JB on blocked_workers.id = JB.worker_id
  WHERE
      JB.job_id = @job_id
  AND JB.task_type = @task_type
);

-- name: WorkersLeftToRunWithWorkerTag :many
SELECT workers.uuid
FROM workers
INNER JOIN worker_tag_membership WTM ON workers.id = WTM.worker_id
WHERE id NOT IN (
  SELECT blocked_workers.id
  FROM workers AS blocked_workers
  INNER JOIN job_blocks JB ON blocked_workers.id = JB.worker_id
  WHERE
      JB.job_id = @job_id
  AND JB.task_type = @task_type
)
AND WTM.worker_tag_id = @worker_tag_id;

-- name: CountTaskFailuresOfWorker :one
SELECT count(TF.task_id) FROM task_failures TF
INNER JOIN tasks T ON TF.task_id = T.id
INNER JOIN jobs J ON T.job_id = J.id
WHERE
    TF.worker_id = @worker_id
AND J.uuid = @jobuuid
AND T.type = @task_type;


-- name: QueryJobTaskSummaries :many
SELECT
  tasks.id,
  tasks.uuid,
  tasks.name,
  tasks.index_in_job,
  tasks.priority,
  tasks.status,
  tasks.type,
  tasks.updated_at,
  tasks.steps_completed,
  tasks.steps_total,
  workers.UUID as workerUUID
FROM tasks
LEFT JOIN jobs ON jobs.id = tasks.job_id
LEFT JOIN workers ON  workers.id = tasks.worker_id
WHERE jobs.uuid=@job_uuid;

-- name: SummarizeJobStatuses :many
SELECT status, count(id) as status_count FROM jobs
GROUP BY status;

-- name: FetchTimedOutTasks :many
SELECT sqlc.embed(tasks),
  -- Cast to remove nullability from the generated structs.
  CAST(jobs.uuid AS VARCHAR(36)) as jobuuid,
  CAST(workers.name AS VARCHAR(64)) as worker_name,
  CAST(workers.uuid AS VARCHAR(36)) as workeruuid
FROM tasks
LEFT JOIN jobs ON jobs.id = tasks.job_id
LEFT JOIN workers ON workers.id = tasks.worker_id
WHERE
    tasks.status = @task_status
AND tasks.last_touched_at <= @untouched_since;

-- name: Test_CountJobs :one
-- Count the number of jobs in the database. Only used in unit tests.
SELECT count(*) AS count FROM jobs;

-- name: Test_CountTasks :one
-- Count the number of tasks in the database. Only used in unit tests.
SELECT count(*) AS count FROM tasks;

-- name: Test_CountTaskFailures :one
-- Count the number of task failures in the database. Only used in unit tests.
SELECT count(*) AS count FROM task_failures;

-- name: Test_FetchTaskFailures :many
-- Fetch all task failures in the database. Only used in unit tests.
SELECT * FROM task_failures;

-- name: Test_FetchLastRendered :many
-- Fetch all 'last rendered' in the database (even though there should only be
-- one at most). Only used in unit tests.
SELECT * FROM last_rendereds;
