-- name: CountTaskUpdates :one
SELECT count(*) as count from task_updates;

-- name: InsertTaskUpdate :exec
INSERT INTO task_updates (
  created_at,
  task_id,
  payload
) VALUES (
  @created_at,
  @task_id,
  @payload
);

-- name: FirstTaskUpdate :one
SELECT * FROM task_updates ORDER BY id LIMIT 1;

-- name: DeleteTaskUpdate :exec
DELETE FROM task_updates WHERE id=@task_update_id;
