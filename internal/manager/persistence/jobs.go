package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/manager/job_compilers"
	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
	"projects.blender.org/studio/flamenco/pkg/api"
)

type Job = sqlc.Job
type Task = sqlc.Task

// TaskJobWorker represents a task, with identifieres for its job and the worker it's assigned to.
type TaskJobWorker struct {
	Task       Task
	JobUUID    string
	WorkerUUID string
}

// TaskJob represents a task, with identifier for its job.
type TaskJob struct {
	Task     Task
	JobUUID  string
	IsActive bool // Whether the worker assigned to this task is actually working on it.
}

type StringInterfaceMap map[string]interface{}
type StringStringMap map[string]string

// Commands is the schema used for (un)marshalling sqlc.Task.Commands.
type Commands []Command

type Command struct {
	Name           string             `json:"name"`
	Parameters     StringInterfaceMap `json:"parameters"`
	TotalStepCount int                `json:"total_step_count"`
}

func (c Commands) Value() (driver.Value, error) {
	return json.Marshal(c)
}
func (c *Commands) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return err
	}
	return nil
}

func (js StringInterfaceMap) Value() (driver.Value, error) {
	return json.Marshal(js)
}
func (js *StringInterfaceMap) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &js)
}

func (js StringStringMap) Value() (driver.Value, error) {
	return json.Marshal(js)
}
func (js *StringStringMap) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &js)
}

// TaskFailure keeps track of which Worker failed which Task.
type TaskFailure = sqlc.TaskFailure

// StoreJob stores an AuthoredJob and its tasks, and saves it to the database.
// The job will be in 'under construction' status. It is up to the caller to transition it to its desired initial status.
func (db *DB) StoreAuthoredJob(ctx context.Context, authoredJob job_compilers.AuthoredJob) error {
	// Serialise the embedded JSON.
	settings, err := json.Marshal(authoredJob.Settings)
	if err != nil {
		return fmt.Errorf("converting job settings to JSON: %w", err)
	}
	metadata, err := json.Marshal(authoredJob.Metadata)
	if err != nil {
		return fmt.Errorf("converting job metadata to JSON: %w", err)
	}

	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		// Create the job itself.
		params := sqlc.CreateJobParams{
			CreatedAt:               db.now(),
			UUID:                    authoredJob.JobID,
			Name:                    authoredJob.Name,
			JobType:                 authoredJob.JobType,
			Priority:                int64(authoredJob.Priority),
			Status:                  authoredJob.Status,
			Settings:                settings,
			Metadata:                metadata,
			StorageShamanCheckoutID: authoredJob.Storage.ShamanCheckoutID,
		}

		if authoredJob.WorkerTagUUID != "" {
			workerTag, err := q.FetchWorkerTagByUUID(ctx, authoredJob.WorkerTagUUID)
			switch {
			case errors.Is(err, sql.ErrNoRows):
				return fmt.Errorf("no worker tag %q found", authoredJob.WorkerTagUUID)
			case err != nil:
				return fmt.Errorf("could not find worker tag %q: %w", authoredJob.WorkerTagUUID, err)
			}
			params.WorkerTagID = sql.NullInt64{Int64: workerTag.ID, Valid: true}
		}

		log.Debug().
			Str("job", params.UUID).
			Str("type", params.JobType).
			Str("name", params.Name).
			Str("status", string(params.Status)).
			Msg("persistence: storing authored job")

		jobID, err := q.CreateJob(ctx, params)
		if err != nil {
			return jobError(err, "storing job")
		}

		err = db.storeAuthoredJobTask(ctx, q, jobID, &authoredJob)
		if err != nil {
			return err
		}
		return nil
	})
}

// StoreAuthoredJobTask is a low-level function that is only used for recreating an existing job's tasks.
// It stores `authoredJob`'s tasks, but attaches them to the already-persisted `job`.
func (db *DB) StoreAuthoredJobTask(
	ctx context.Context,
	job *Job,
	authoredJob *job_compilers.AuthoredJob,
) error {
	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return db.storeAuthoredJobTask(ctx, q, int64(job.ID), authoredJob)
	})
}

// storeAuthoredJobTask stores the tasks of the authored job.
// Note that this function does NOT commit the database transaction. That is up
// to the caller.
func (db *DB) storeAuthoredJobTask(
	ctx context.Context,
	q *sqlc.Queries,
	jobID int64,
	authoredJob *job_compilers.AuthoredJob,
) error {
	type TaskInfo struct {
		ID   int64
		UUID string
		Name string
	}

	// Give every task the same creation timestamp.
	now := db.now()

	uuidToTask := make(map[string]TaskInfo)
	for taskIndex, authoredTask := range authoredJob.Tasks {
		// Marshal commands to JSON.
		var commands []Command
		taskStepCount := 0
		for _, authoredCommand := range authoredTask.Commands {
			commands = append(commands, Command{
				Name:           authoredCommand.Name,
				Parameters:     StringInterfaceMap(authoredCommand.Parameters),
				TotalStepCount: authoredCommand.TotalStepCount,
			})
			// When the command doesn't have/support steps, the command itself is
			// counted as one step.
			taskStepCount += max(1, authoredCommand.TotalStepCount)
		}
		commandsJSON, err := json.Marshal(commands)
		if err != nil {
			return fmt.Errorf("could not convert commands of task %q to JSON: %w",
				authoredTask.Name, err)
		}

		taskParams := sqlc.CreateTaskParams{
			CreatedAt:  now,
			Name:       authoredTask.Name,
			Type:       authoredTask.Type,
			UUID:       authoredTask.UUID,
			JobID:      jobID,
			IndexInJob: int64(taskIndex + 1), // indexInJob is base-1.
			Priority:   int64(authoredTask.Priority),
			Status:     api.TaskStatusQueued,
			Commands:   commandsJSON,
			StepsTotal: int64(taskStepCount),
			// dependencies are stored below.
		}

		log.Debug().
			Str("task", taskParams.UUID).
			Str("type", taskParams.Type).
			Str("name", taskParams.Name).
			Str("status", string(taskParams.Status)).
			Msg("persistence: storing authored task")

		taskID, err := q.CreateTask(ctx, taskParams)
		if err != nil {
			return taskError(err, "storing task: %v", err)
		}

		uuidToTask[authoredTask.UUID] = TaskInfo{
			ID:   taskID,
			UUID: taskParams.UUID,
			Name: taskParams.Name,
		}
	}

	// Store the dependencies between tasks.
	for _, authoredTask := range authoredJob.Tasks {
		if len(authoredTask.Dependencies) == 0 {
			continue
		}

		taskInfo, ok := uuidToTask[authoredTask.UUID]
		if !ok {
			return taskError(nil, "unable to find task %q in the database, even though it was just authored", authoredTask.UUID)
		}

		deps := make([]*TaskInfo, len(authoredTask.Dependencies))
		for idx, authoredDep := range authoredTask.Dependencies {
			depTask, ok := uuidToTask[authoredDep.UUID]
			if !ok {
				return taskError(nil, "finding task with UUID %q; a task depends on a task that is not part of this job", authoredDep.UUID)
			}

			err := q.StoreTaskDependency(ctx, sqlc.StoreTaskDependencyParams{
				TaskID:       taskInfo.ID,
				DependencyID: depTask.ID,
			})
			if err != nil {
				return taskError(err, "error storing task %q depending on task %q", authoredTask.UUID, depTask.UUID)
			}

			deps[idx] = &depTask
		}

		if log.Debug().Enabled() {
			depNames := make([]string, len(deps))
			for i, dep := range deps {
				depNames[i] = dep.Name
			}
			log.Debug().
				Str("task", taskInfo.UUID).
				Str("name", taskInfo.Name).
				Strs("dependencies", depNames).
				Msg("persistence: storing authored task dependencies")
		}
	}

	return nil
}

// FetchJob fetches a single job, without fetching its tasks.
func (db *DB) FetchJob(ctx context.Context, jobUUID string) (*Job, error) {
	var job Job
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		job, err = q.FetchJob(ctx, jobUUID)
		return
	})

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrJobNotFound
	case err != nil:
		return nil, jobError(err, "fetching job")
	}

	return &job, nil
}

// FetchJob fetches a single job by its database ID, without fetching its tasks.
func (db *DB) FetchJobByID(ctx context.Context, jobID int64) (*Job, error) {
	var job Job
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		job, err = q.FetchJobByID(ctx, jobID)
		return
	})

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrJobNotFound
	case err != nil:
		return nil, jobError(err, "fetching job")
	}

	return &job, nil
}

func (db *DB) FetchJobs(ctx context.Context) ([]*Job, error) {
	var sqlcJobs []sqlc.Job

	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		sqlcJobs, err = q.FetchJobs(ctx)
		return
	})
	if err != nil {
		return nil, jobError(err, "fetching all jobs")
	}

	// TODO: just return []Job instead of converting the array.
	jobPointers := make([]*Job, len(sqlcJobs))
	for index := range sqlcJobs {
		jobPointers[index] = &sqlcJobs[index]
	}

	return jobPointers, nil
}

// FetchJobShamanCheckoutID fetches the job's Shaman Checkout ID.
func (db *DB) FetchJobShamanCheckoutID(ctx context.Context, jobUUID string) (string, error) {
	var checkoutID string
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		checkoutID, err = q.FetchJobShamanCheckoutID(ctx, jobUUID)
		return
	})

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", ErrJobNotFound
	case err != nil:
		return "", jobError(err, "fetching job")
	}
	return checkoutID, nil
}

// DeleteJob deletes a job from the database.
// The deletion cascades to its tasks and other job-related tables.
func (db *DB) DeleteJob(ctx context.Context, jobUUID string) error {
	// As a safety measure, refuse to delete jobs unless foreign key constraints are active.
	fkEnabled, err := db.areForeignKeysEnabled(ctx)
	switch {
	case err != nil:
		return err
	case !fkEnabled:
		return ErrDeletingWithoutFK
	}

	err = db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.DeleteJob(ctx, jobUUID)
	})

	return jobError(err, "deleting job")
}

// RequestJobDeletion sets the job's "DeletionRequestedAt" field to "now".
func (db *DB) RequestJobDeletion(ctx context.Context, j *Job) error {
	params := sqlc.RequestJobDeletionParams{
		Now:   db.nowNullable(),
		JobID: int64(j.ID),
	}

	log.Trace().
		Str("job", j.UUID).
		Time("deletedAt", params.Now.Time).
		Msg("database: marking job as deletion-requested")

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.RequestJobDeletion(ctx, params)
	})
	if err != nil {
		return jobError(err, "queueing job for deletion")
	}

	// Update the given job itself, so we don't have to re-fetch it from the database.
	j.DeleteRequestedAt = params.Now

	return nil
}

// RequestJobMassDeletion sets multiple job's "DeletionRequestedAt" field to "now".
// The list of affected job UUIDs is returned.
func (db *DB) RequestJobMassDeletion(ctx context.Context, lastUpdatedMax time.Time) ([]string, error) {
	var uuids []string

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		// In order to be able to report which jobs were affected, first fetch the
		// list of jobs, then update them.
		var err error
		uuids, err = q.FetchJobUUIDsUpdatedBefore(ctx, sql.NullTime{
			Time:  lastUpdatedMax,
			Valid: true,
		})
		switch {
		case err != nil:
			return jobError(err, "fetching jobs by last-modified timestamp")
		case len(uuids) == 0:
			return ErrJobNotFound
		}

		// Update the selected jobs.
		params := sqlc.RequestMassJobDeletionParams{
			Now:   db.nowNullable(),
			UUIDs: uuids,
		}
		if err := q.RequestMassJobDeletion(ctx, params); err != nil {
			return jobError(err, "marking jobs as deletion-requested")
		}
		return nil
	})

	return uuids, err
}

func (db *DB) FetchJobsDeletionRequested(ctx context.Context) ([]string, error) {
	var uuids []string
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		uuids, err = q.FetchJobsDeletionRequested(ctx)
		return
	})
	return uuids, jobError(err, "fetching jobs marked for deletion")
}

func (db *DB) FetchJobsInStatus(ctx context.Context, jobStatuses ...api.JobStatus) ([]*Job, error) {
	var jobs []sqlc.Job
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		jobs, err = q.FetchJobsInStatus(ctx, jobStatuses)
		return
	})
	if err != nil {
		return nil, jobError(err, "fetching jobs in status %q", jobStatuses)
	}

	// TODO: just return []Job instead of converting the array.
	pointers := make([]*Job, len(jobs))
	for index := range jobs {
		pointers[index] = &jobs[index]
	}

	return pointers, nil
}

// SaveJobStatus saves the job's Status and Activity fields.
func (db *DB) SaveJobStatus(ctx context.Context, j *Job) error {
	params := sqlc.SaveJobStatusParams{
		Now:      db.nowNullable(),
		ID:       int64(j.ID),
		Status:   j.Status,
		Activity: j.Activity,
	}

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.SaveJobStatus(ctx, params)
	})
	return jobError(err, "saving job status")
}

// SaveJobPriority saves the job's Priority field.
func (db *DB) SaveJobPriority(ctx context.Context, j *Job) error {
	params := sqlc.SaveJobPriorityParams{
		Now:      db.nowNullable(),
		ID:       int64(j.ID),
		Priority: int64(j.Priority),
	}

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.SaveJobPriority(ctx, params)
	})
	return jobError(err, "saving job priority")
}

// SaveJobWorkerTag saves the job's WorkerTagID field.
func (db *DB) SaveJobWorkerTag(ctx context.Context, j *Job) error {
	params := sqlc.SaveJobWorkerTagParams{
		Now:         db.nowNullable(),
		ID:          int64(j.ID),
		WorkerTagID: j.WorkerTagID,
	}

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.SaveJobWorkerTag(ctx, params)
	})
	return jobError(err, "saving job worker tag")
}

// SaveJobStorageInfo saves the job's Storage field.
// NOTE: this function does NOT update the job's `UpdatedAt` field. This is
// necessary for `cmd/shaman-checkout-id-setter` to do its work quietly.
func (db *DB) SaveJobStorageInfo(ctx context.Context, j *Job) error {
	params := sqlc.SaveJobStorageInfoParams{
		ID:                      int64(j.ID),
		StorageShamanCheckoutID: j.StorageShamanCheckoutID,
	}

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.SaveJobStorageInfo(ctx, params)
	})
	return jobError(err, "saving job storage")
}

func (db *DB) FetchTask(ctx context.Context, taskUUID string) (TaskJobWorker, error) {
	var taskRow sqlc.FetchTaskRow
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		taskRow, err = q.FetchTask(ctx, taskUUID)
		return
	})
	if err != nil {
		return TaskJobWorker{}, taskError(err, "fetching task %s", taskUUID)
	}

	taskJobWorker := TaskJobWorker{
		Task:       taskRow.Task,
		JobUUID:    taskRow.JobUUID.String,
		WorkerUUID: taskRow.WorkerUUID.String,
	}

	return taskJobWorker, nil
}

// FetchTaskJobUUID fetches the job UUID of the given task.
func (db *DB) FetchTaskJobUUID(ctx context.Context, taskUUID string) (string, error) {
	var jobUUID sql.NullString
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		jobUUID, err = q.FetchTaskJobUUID(ctx, taskUUID)
		return
	})

	if err != nil {
		return "", taskError(err, "fetching job UUID of task %s", taskUUID)
	}
	if !jobUUID.Valid {
		return "", PersistenceError{Message: fmt.Sprintf("unable to find job of task %s", taskUUID)}
	}
	return jobUUID.String, nil
}

// SaveTask updates a task that already exists in the database.
// This function is not used by the Flamenco API, only by unit tests.
func (db *DB) SaveTask(ctx context.Context, t *Task) error {
	if t.ID == 0 {
		panic(fmt.Errorf("cannot use this function to insert a task"))
	}

	commandsJSON, err := json.Marshal(t.Commands)
	if err != nil {
		return fmt.Errorf("cannot convert commands to JSON: %w", err)
	}

	param := sqlc.UpdateTaskParams{
		UpdatedAt:      db.nowNullable(),
		Name:           t.Name,
		Type:           t.Type,
		Priority:       t.Priority,
		Status:         t.Status,
		Commands:       commandsJSON,
		Activity:       t.Activity,
		ID:             t.ID,
		WorkerID:       t.WorkerID,
		LastTouchedAt:  t.LastTouchedAt,
		StepsTotal:     t.StepsTotal,
		StepsCompleted: t.StepsCompleted,
	}

	err = db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.UpdateTask(ctx, param)
	})
	return taskError(err, "updating task")
}

func (db *DB) SaveTaskStatus(ctx context.Context, t *Task) error {
	params := sqlc.UpdateTaskStatusParams{
		UpdatedAt: db.nowNullable(),
		Status:    t.Status,
		ID:        int64(t.ID),
	}

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.UpdateTaskStatus(ctx, params)
	})
	return taskError(err, "saving task status")
}

func (db *DB) SaveTaskActivity(ctx context.Context, t *Task) error {
	params := sqlc.UpdateTaskActivityParams{
		UpdatedAt: db.nowNullable(),
		Activity:  t.Activity,
		ID:        int64(t.ID),
	}

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.UpdateTaskActivity(ctx, params)
	})
	return taskError(err, "saving task activity")
}

// SaveTaskStepsCompleted updates the task's step completion counter,
// and updates the job for the new completion count as well.
func (db *DB) SaveTaskStepsCompleted(ctx context.Context, jobID, taskID int64, stepsCompleted int64) error {
	now := db.nowNullable()

	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		err := q.UpdateTaskStepsCompleted(ctx, sqlc.UpdateTaskStepsCompletedParams{
			UpdatedAt:      now,
			ID:             taskID,
			StepsCompleted: stepsCompleted,
		})
		if err != nil {
			return taskError(err, "saving task steps completed")
		}

		err = q.UpdateJobStepsCompleted(ctx, sqlc.UpdateJobStepsCompletedParams{
			UpdatedAt: now,
			ID:        jobID,
		})
		if err != nil {
			return jobError(err, "updating job for task steps completed")
		}
		return nil
	})
}

// TaskAssignToWorker assigns the given task to the given worker.
// This function is only used by unit tests. During normal operation, Flamenco
// uses the code in task_scheduler.go to assign tasks to workers.
func (db *DB) TaskAssignToWorker(ctx context.Context, t *Task, w *Worker) error {
	params := sqlc.TaskAssignToWorkerParams{
		UpdatedAt: db.nowNullable(),
		WorkerID:  sql.NullInt64{Int64: w.ID, Valid: true},
		ID:        t.ID,
	}

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.TaskAssignToWorker(ctx, params)
	})

	if err != nil {
		return taskError(err, "assigning task %s to worker %s", t.UUID, w.UUID)
	}

	t.WorkerID = params.WorkerID
	return nil
}

func (db *DB) FetchTasksOfWorkerInStatus(ctx context.Context, worker *Worker, taskStatus api.TaskStatus) ([]TaskJob, error) {
	params := sqlc.FetchTasksOfWorkerInStatusParams{
		WorkerID: sql.NullInt64{
			Int64: int64(worker.ID),
			Valid: true,
		},
		TaskStatus: taskStatus,
	}

	var rows []sqlc.FetchTasksOfWorkerInStatusRow
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		rows, err = q.FetchTasksOfWorkerInStatus(ctx, params)
		return
	})

	if err != nil {
		return nil, taskError(err, "finding tasks of worker %s in status %q", worker.UUID, taskStatus)
	}

	result := make([]TaskJob, len(rows))
	for i := range rows {
		result[i].Task = rows[i].Task
		result[i].JobUUID = rows[i].JobUUID
	}
	return result, nil
}

func (db *DB) FetchTasksOfWorkerInStatusOfJob(ctx context.Context, worker *Worker, taskStatus api.TaskStatus, jobUUID string) ([]*Task, error) {
	var rows []sqlc.FetchTasksOfWorkerInStatusOfJobRow
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		rows, err = q.FetchTasksOfWorkerInStatusOfJob(ctx, sqlc.FetchTasksOfWorkerInStatusOfJobParams{
			WorkerID: sql.NullInt64{
				Int64: int64(worker.ID),
				Valid: true,
			},
			JobUUID:    jobUUID,
			TaskStatus: taskStatus,
		})
		return err
	})
	if err != nil {
		return nil, taskError(err, "finding tasks of worker %s in status %q and job %s", worker.UUID, taskStatus, jobUUID)
	}

	// TODO: just return []Task instead of creating an array of pointers.
	result := make([]*Task, len(rows))
	for i := range rows {
		result[i] = &rows[i].Task
	}
	return result, nil
}

func (db *DB) JobHasTasksInStatus(ctx context.Context, job *Job, taskStatus api.TaskStatus) (bool, error) {
	var count int64
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		count, err = q.JobCountTasksInStatus(ctx, sqlc.JobCountTasksInStatusParams{
			JobID:      int64(job.ID),
			TaskStatus: taskStatus,
		})
		return
	})

	if err != nil {
		return false, taskError(err, "counting tasks of job %s in status %q", job.UUID, taskStatus)
	}
	return count > 0, nil
}

// CountTasksOfJobInStatus counts the number of tasks in the job.
// It returns two counts, one is the number of tasks in the given statuses, the
// other is the total number of tasks of the job.
func (db *DB) CountTasksOfJobInStatus(
	ctx context.Context,
	job *Job,
	taskStatuses ...api.TaskStatus,
) (numInStatus, numTotal int, err error) {
	var results []sqlc.JobCountTaskStatusesRow
	err = db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		results, err = q.JobCountTaskStatuses(ctx, int64(job.ID))
		return
	})
	if err != nil {
		return 0, 0, jobError(err, "count tasks of job %s in status %q", job.UUID, taskStatuses)
	}

	// Create lookup table for which statuses to count.
	countStatus := map[api.TaskStatus]bool{}
	for _, status := range taskStatuses {
		countStatus[status] = true
	}

	// Count the number of tasks per status.
	for _, result := range results {
		if countStatus[api.TaskStatus(result.Status)] {
			numInStatus += int(result.NumTasks)
		}
		numTotal += int(result.NumTasks)
	}

	return
}

// FetchTaskIDsOfJob returns all tasks of the given job.
func (db *DB) FetchTasksOfJob(ctx context.Context, job *Job) ([]TaskJobWorker, error) {
	var rows []sqlc.FetchTasksOfJobRow
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		rows, err = q.FetchTasksOfJob(ctx, int64(job.ID))
		return
	})

	if err != nil {
		return nil, taskError(err, "fetching tasks of job %s", job.UUID)
	}

	result := make([]TaskJobWorker, len(rows))
	for i := range rows {
		result[i].Task = rows[i].Task
		result[i].JobUUID = job.UUID
		result[i].WorkerUUID = rows[i].WorkerUUID.String
	}
	return result, nil
}

// FetchTasksOfJobInStatus returns those tasks of the given job that have any of the given statuses.
func (db *DB) FetchTasksOfJobInStatus(ctx context.Context, job *Job, taskStatuses ...api.TaskStatus) ([]TaskJobWorker, error) {
	var rows []sqlc.FetchTasksOfJobInStatusRow
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		rows, err = q.FetchTasksOfJobInStatus(ctx, sqlc.FetchTasksOfJobInStatusParams{
			JobID:      int64(job.ID),
			TaskStatus: taskStatuses,
		})
		return
	})
	if err != nil {
		return nil, taskError(err, "fetching tasks of job %s in status %q", job.UUID, taskStatuses)
	}

	result := make([]TaskJobWorker, len(rows))
	for i := range rows {
		result[i].Task = rows[i].Task
		result[i].JobUUID = job.UUID
		result[i].WorkerUUID = rows[i].WorkerUUID.String
	}
	return result, nil
}

// UpdateJobsTaskStatuses updates the status & activity of all tasks of `job`.
func (db *DB) UpdateJobsTaskStatuses(ctx context.Context, job *Job,
	taskStatus api.TaskStatus, activity string) error {

	if taskStatus == "" {
		return taskError(nil, "empty status not allowed")
	}

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.UpdateJobsTaskStatuses(ctx, sqlc.UpdateJobsTaskStatusesParams{
			UpdatedAt: db.nowNullable(),
			Status:    taskStatus,
			Activity:  activity,
			JobID:     int64(job.ID),
		})
	})

	return taskError(err, "updating status of all tasks of job %s", job.UUID)
}

// UpdateJobsTaskStatusesConditional updates the status & activity of the tasks of `job`,
// limited to those tasks with status in `statusesToUpdate`.
func (db *DB) UpdateJobsTaskStatusesConditional(ctx context.Context, job *Job,
	statusesToUpdate []api.TaskStatus, taskStatus api.TaskStatus, activity string) error {

	if taskStatus == "" {
		return taskError(nil, "empty status not allowed")
	}

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.UpdateJobsTaskStatusesConditional(ctx, sqlc.UpdateJobsTaskStatusesConditionalParams{
			UpdatedAt:        db.nowNullable(),
			Status:           taskStatus,
			Activity:         activity,
			JobID:            int64(job.ID),
			StatusesToUpdate: statusesToUpdate,
		})
	})

	return taskError(err, "updating status of all tasks in status %v of job %s", statusesToUpdate, job.UUID)
}

// UpdateJobsTaskStepCounts goes over all tasks of the job, and resets their steps_completed
// based on their status.
func (db *DB) UpdateJobsTaskStepCounts(ctx context.Context, jobID int64) error {
	now := db.nowNullable()
	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		err := q.UpdateJobsTaskStepCountsComplete(ctx, sqlc.UpdateJobsTaskStepCountsCompleteParams{
			UpdatedAt:        now,
			JobID:            jobID,
			StatusesToUpdate: []api.TaskStatus{api.TaskStatusCompleted},
		})
		if err != nil {
			return taskError(err, "updating completed step count on job %q", jobID)
		}

		err = q.UpdateJobsTaskStepCountsZero(ctx, sqlc.UpdateJobsTaskStepCountsZeroParams{
			UpdatedAt:        now,
			JobID:            jobID,
			StatusesToUpdate: []api.TaskStatus{api.TaskStatusQueued},
		})
		if err != nil {
			return taskError(err, "updating completed step count on job %q", jobID)
		}

		err = q.UpdateJobStepsCompleted(ctx, sqlc.UpdateJobStepsCompletedParams{
			UpdatedAt: now,
			ID:        jobID,
		})
		if err != nil {
			return jobError(err, "updating job for task steps completed")
		}
		return nil
	})
}

// TaskTouchedByWorker marks the task as 'touched' by a worker. This is used for timeout detection.
func (db *DB) TaskTouchedByWorker(ctx context.Context, taskUUID string) error {
	now := db.nowNullable()

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.TaskTouchedByWorker(ctx, sqlc.TaskTouchedByWorkerParams{
			UpdatedAt:     now,
			LastTouchedAt: now,
			UUID:          taskUUID,
		})
	})
	return taskError(err, "saving task 'last touched at'")
}

// AddWorkerToTaskFailedList records that the given worker failed the given task.
// This information is not used directly by the task scheduler. It's used to
// determine whether there are any workers left to perform this task, and thus
// whether it should be hard- or soft-failed.
//
// Calling this multiple times with the same task/worker is a no-op.
//
// Returns the new number of workers that failed this task.
func (db *DB) AddWorkerToTaskFailedList(ctx context.Context, t *Task, w *Worker) (numFailed int, err error) {
	var numFailed64 int64
	err = db.queriesRW(ctx, func(q *sqlc.Queries) error {
		err := q.AddWorkerToTaskFailedList(ctx, sqlc.AddWorkerToTaskFailedListParams{
			CreatedAt: db.nowNullable().Time,
			TaskID:    int64(t.ID),
			WorkerID:  int64(w.ID),
		})
		if err != nil {
			return err
		}

		numFailed64, err = q.CountWorkersFailingTask(ctx, int64(t.ID))
		return err
	})
	if err != nil {
		return 0, err
	}

	// Integer literals are of type `int`, so that's just a bit nicer to work with
	// than `int64`.
	if numFailed64 > math.MaxInt32 {
		log.Warn().Int64("numFailed", numFailed64).Msg("number of failed workers is crazy high, something is wrong here")
		return math.MaxInt32, nil
	}
	return int(numFailed64), nil
}

// ClearFailureListOfTask clears the list of workers that failed this task.
func (db *DB) ClearFailureListOfTask(ctx context.Context, t *Task) error {
	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.ClearFailureListOfTask(ctx, int64(t.ID))
	})
}

// ClearFailureListOfJob en-mass, for all tasks of this job, clears the list of
// workers that failed those tasks.
func (db *DB) ClearFailureListOfJob(ctx context.Context, j *Job) error {
	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.ClearFailureListOfJob(ctx, int64(j.ID))
	})
}

func (db *DB) FetchTaskFailureList(ctx context.Context, t *Task) ([]*Worker, error) {
	var failureList []sqlc.FetchTaskFailureListRow

	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		failureList, err = q.FetchTaskFailureList(ctx, int64(t.ID))
		return
	})
	if err != nil {
		return nil, err
	}

	workers := make([]*Worker, len(failureList))
	for idx := range failureList {
		workers[idx] = &failureList[idx].Worker
	}
	return workers, nil
}
