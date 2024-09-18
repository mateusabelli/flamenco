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
	"gorm.io/gorm"

	"projects.blender.org/studio/flamenco/internal/manager/job_compilers"
	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
	"projects.blender.org/studio/flamenco/pkg/api"
)

type Job struct {
	Model
	UUID string `gorm:"type:char(36);default:'';unique;index"`

	Name     string        `gorm:"type:varchar(64);default:''"`
	JobType  string        `gorm:"type:varchar(32);default:''"`
	Priority int           `gorm:"type:smallint;default:0"`
	Status   api.JobStatus `gorm:"type:varchar(32);default:''"`
	Activity string        `gorm:"type:varchar(255);default:''"`

	Settings StringInterfaceMap `gorm:"type:jsonb"`
	Metadata StringStringMap    `gorm:"type:jsonb"`

	DeleteRequestedAt sql.NullTime

	Storage JobStorageInfo `gorm:"embedded;embeddedPrefix:storage_"`

	WorkerTagID *uint
	WorkerTag   *WorkerTag `gorm:"foreignkey:WorkerTagID;references:ID;constraint:OnDelete:SET NULL"`
}

type StringInterfaceMap map[string]interface{}
type StringStringMap map[string]string

// DeleteRequested returns whether deletion of this job was requested.
func (j *Job) DeleteRequested() bool {
	return j.DeleteRequestedAt.Valid
}

// JobStorageInfo contains info about where the job files are stored. It is
// intended to be used when removing a job, which may include the removal of its
// files.
type JobStorageInfo struct {
	// ShamanCheckoutID is only set when the job was actually using Shaman storage.
	ShamanCheckoutID string `gorm:"type:varchar(255);default:''"`
}

type Task struct {
	Model
	UUID string `gorm:"type:char(36);default:'';unique;index"`

	Name     string         `gorm:"type:varchar(64);default:''"`
	Type     string         `gorm:"type:varchar(32);default:''"`
	JobID    uint           `gorm:"default:0"`
	Job      *Job           `gorm:"foreignkey:JobID;references:ID;constraint:OnDelete:CASCADE"`
	JobUUID  string         `gorm:"-"` // Fetched by SQLC, handled by GORM in Task.AfterFind()
	Priority int            `gorm:"type:smallint;default:50"`
	Status   api.TaskStatus `gorm:"type:varchar(16);default:''"`

	// Which worker is/was working on this.
	WorkerID      *uint
	Worker        *Worker   `gorm:"foreignkey:WorkerID;references:ID;constraint:OnDelete:SET NULL"`
	WorkerUUID    string    `gorm:"-"`     // Fetched by SQLC, handled by GORM in Task.AfterFind()
	LastTouchedAt time.Time `gorm:"index"` // Should contain UTC timestamps.

	// Dependencies are tasks that need to be completed before this one can run.
	Dependencies []*Task `gorm:"many2many:task_dependencies;constraint:OnDelete:CASCADE"`

	Commands Commands `gorm:"type:jsonb"`
	Activity string   `gorm:"type:varchar(255);default:''"`
}

// AfterFind updates the task JobUUID and WorkerUUID fields from its job/worker, if known.
func (t *Task) AfterFind(tx *gorm.DB) error {
	if t.JobUUID == "" && t.Job != nil {
		t.JobUUID = t.Job.UUID
	}
	if t.WorkerUUID == "" && t.Worker != nil {
		t.WorkerUUID = t.Worker.UUID
	}
	return nil
}

type Commands []Command

type Command struct {
	Name       string             `json:"name"`
	Parameters StringInterfaceMap `json:"parameters"`
}

func (c Commands) Value() (driver.Value, error) {
	return json.Marshal(c)
}
func (c *Commands) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &c)
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
type TaskFailure struct {
	// Don't include the standard Gorm ID, UpdatedAt, or DeletedAt fields, as they're useless here.
	// Entries will never be updated, and should never be soft-deleted but just purged from existence.
	CreatedAt time.Time
	TaskID    uint    `gorm:"primaryKey;autoIncrement:false"`
	Task      *Task   `gorm:"foreignkey:TaskID;references:ID;constraint:OnDelete:CASCADE"`
	WorkerID  uint    `gorm:"primaryKey;autoIncrement:false"`
	Worker    *Worker `gorm:"foreignkey:WorkerID;references:ID;constraint:OnDelete:CASCADE"`
}

// StoreJob stores an AuthoredJob and its tasks, and saves it to the database.
// The job will be in 'under construction' status. It is up to the caller to transition it to its desired initial status.
func (db *DB) StoreAuthoredJob(ctx context.Context, authoredJob job_compilers.AuthoredJob) error {

	// Run all queries in a single transaction.
	qtx, err := db.queriesWithTX()
	if err != nil {
		return err
	}
	defer qtx.rollback()

	// Serialise the embedded JSON.
	settings, err := json.Marshal(authoredJob.Settings)
	if err != nil {
		return fmt.Errorf("converting job settings to JSON: %w", err)
	}
	metadata, err := json.Marshal(authoredJob.Metadata)
	if err != nil {
		return fmt.Errorf("converting job metadata to JSON: %w", err)
	}

	// Create the job itself.
	params := sqlc.CreateJobParams{
		CreatedAt:               db.gormDB.NowFunc(),
		UUID:                    authoredJob.JobID,
		Name:                    authoredJob.Name,
		JobType:                 authoredJob.JobType,
		Priority:                int64(authoredJob.Priority),
		Status:                  string(authoredJob.Status),
		Settings:                settings,
		Metadata:                metadata,
		StorageShamanCheckoutID: authoredJob.Storage.ShamanCheckoutID,
	}

	if authoredJob.WorkerTagUUID != "" {
		dbTag, err := qtx.queries.FetchWorkerTagByUUID(ctx, authoredJob.WorkerTagUUID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("no worker tag %q found", authoredJob.WorkerTagUUID)
		case err != nil:
			return fmt.Errorf("could not find worker tag %q: %w", authoredJob.WorkerTagUUID, err)
		}
		params.WorkerTagID = sql.NullInt64{Int64: dbTag.WorkerTag.ID, Valid: true}
	}

	log.Debug().
		Str("job", params.UUID).
		Str("type", params.JobType).
		Str("name", params.Name).
		Str("status", params.Status).
		Msg("persistence: storing authored job")

	jobID, err := qtx.queries.CreateJob(ctx, params)
	if err != nil {
		return jobError(err, "storing job")
	}

	err = db.storeAuthoredJobTaks(ctx, qtx, jobID, &authoredJob)
	if err != nil {
		return err
	}

	return qtx.commit()
}

// StoreAuthoredJobTaks is a low-level function that is only used for recreating an existing job's tasks.
// It stores `authoredJob`'s tasks, but attaches them to the already-persisted `job`.
func (db *DB) StoreAuthoredJobTaks(
	ctx context.Context,
	job *Job,
	authoredJob *job_compilers.AuthoredJob,
) error {
	qtx, err := db.queriesWithTX()
	if err != nil {
		return err
	}
	defer qtx.rollback()

	err = db.storeAuthoredJobTaks(ctx, qtx, int64(job.ID), authoredJob)
	if err != nil {
		return err
	}

	return qtx.commit()
}

// storeAuthoredJobTaks stores the tasks of the authored job.
// Note that this function does NOT commit the database transaction. That is up
// to the caller.
func (db *DB) storeAuthoredJobTaks(
	ctx context.Context,
	qtx *queriesTX,
	jobID int64,
	authoredJob *job_compilers.AuthoredJob,
) error {
	type TaskInfo struct {
		ID   int64
		UUID string
		Name string
	}

	// Give every task the same creation timestamp.
	now := db.gormDB.NowFunc()

	uuidToTask := make(map[string]TaskInfo)
	for _, authoredTask := range authoredJob.Tasks {
		// Marshal commands to JSON.
		var commands []Command
		for _, authoredCommand := range authoredTask.Commands {
			commands = append(commands, Command{
				Name:       authoredCommand.Name,
				Parameters: StringInterfaceMap(authoredCommand.Parameters),
			})
		}
		commandsJSON, err := json.Marshal(commands)
		if err != nil {
			return fmt.Errorf("could not convert commands of task %q to JSON: %w",
				authoredTask.Name, err)
		}

		taskParams := sqlc.CreateTaskParams{
			CreatedAt: now,
			Name:      authoredTask.Name,
			Type:      authoredTask.Type,
			UUID:      authoredTask.UUID,
			JobID:     jobID,
			Priority:  int64(authoredTask.Priority),
			Status:    string(api.TaskStatusQueued),
			Commands:  commandsJSON,
			// dependencies are stored below.
		}

		log.Debug().
			Str("task", taskParams.UUID).
			Str("type", taskParams.Type).
			Str("name", taskParams.Name).
			Str("status", string(taskParams.Status)).
			Msg("persistence: storing authored task")

		taskID, err := qtx.queries.CreateTask(ctx, taskParams)
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

			err := qtx.queries.StoreTaskDependency(ctx, sqlc.StoreTaskDependencyParams{
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
	queries := db.queries()

	sqlcJob, err := queries.FetchJob(ctx, jobUUID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrJobNotFound
	case err != nil:
		return nil, jobError(err, "fetching job")
	}

	gormJob, err := convertSqlcJob(sqlcJob)
	if err != nil {
		return nil, err
	}

	if sqlcJob.WorkerTagID.Valid {
		workerTag, err := fetchWorkerTagByID(db.gormDB, uint(sqlcJob.WorkerTagID.Int64))
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrWorkerTagNotFound
		case err != nil:
			return nil, workerTagError(err, "fetching worker tag of job")
		}
		gormJob.WorkerTag = workerTag
	}

	return &gormJob, nil
}

// FetchJobShamanCheckoutID fetches the job's Shaman Checkout ID.
func (db *DB) FetchJobShamanCheckoutID(ctx context.Context, jobUUID string) (string, error) {
	queries := db.queries()

	checkoutID, err := queries.FetchJobShamanCheckoutID(ctx, jobUUID)
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

	queries := db.queries()

	if err := queries.DeleteJob(ctx, jobUUID); err != nil {
		return jobError(err, "deleting job")
	}
	return nil
}

// RequestJobDeletion sets the job's "DeletionRequestedAt" field to "now".
func (db *DB) RequestJobDeletion(ctx context.Context, j *Job) error {
	queries := db.queries()

	// Update the given job itself, so we don't have to re-fetch it from the database.
	j.DeleteRequestedAt = db.now()

	params := sqlc.RequestJobDeletionParams{
		Now:   j.DeleteRequestedAt,
		JobID: int64(j.ID),
	}

	log.Trace().
		Str("job", j.UUID).
		Time("deletedAt", params.Now.Time).
		Msg("database: marking job as deletion-requested")
	if err := queries.RequestJobDeletion(ctx, params); err != nil {
		return jobError(err, "queueing job for deletion")
	}
	return nil
}

// RequestJobMassDeletion sets multiple job's "DeletionRequestedAt" field to "now".
// The list of affected job UUIDs is returned.
func (db *DB) RequestJobMassDeletion(ctx context.Context, lastUpdatedMax time.Time) ([]string, error) {
	queries := db.queries()

	// In order to be able to report which jobs were affected, first fetch the
	// list of jobs, then update them.
	uuids, err := queries.FetchJobUUIDsUpdatedBefore(ctx, sql.NullTime{
		Time:  lastUpdatedMax,
		Valid: true,
	})
	switch {
	case err != nil:
		return nil, jobError(err, "fetching jobs by last-modified timestamp")
	case len(uuids) == 0:
		return nil, ErrJobNotFound
	}

	// Update the selected jobs.
	params := sqlc.RequestMassJobDeletionParams{
		Now:   db.now(),
		UUIDs: uuids,
	}
	if err := queries.RequestMassJobDeletion(ctx, params); err != nil {
		return nil, jobError(err, "marking jobs as deletion-requested")
	}

	return uuids, nil
}

func (db *DB) FetchJobsDeletionRequested(ctx context.Context) ([]string, error) {
	queries := db.queries()

	uuids, err := queries.FetchJobsDeletionRequested(ctx)
	if err != nil {
		return nil, jobError(err, "fetching jobs marked for deletion")
	}
	return uuids, nil
}

func (db *DB) FetchJobsInStatus(ctx context.Context, jobStatuses ...api.JobStatus) ([]*Job, error) {
	queries := db.queries()

	statuses := []string{}
	for _, status := range jobStatuses {
		statuses = append(statuses, string(status))
	}

	sqlcJobs, err := queries.FetchJobsInStatus(ctx, statuses)
	if err != nil {
		return nil, jobError(err, "fetching jobs in status %q", jobStatuses)
	}

	var jobs []*Job
	for index := range sqlcJobs {
		job, err := convertSqlcJob(sqlcJobs[index])
		if err != nil {
			return nil, jobError(err, "converting fetched jobs in status %q", jobStatuses)
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// SaveJobStatus saves the job's Status and Activity fields.
func (db *DB) SaveJobStatus(ctx context.Context, j *Job) error {
	queries := db.queries()

	params := sqlc.SaveJobStatusParams{
		Now:      db.now(),
		ID:       int64(j.ID),
		Status:   string(j.Status),
		Activity: j.Activity,
	}

	err := queries.SaveJobStatus(ctx, params)
	if err != nil {
		return jobError(err, "saving job status")
	}
	return nil
}

// SaveJobPriority saves the job's Priority field.
func (db *DB) SaveJobPriority(ctx context.Context, j *Job) error {
	queries := db.queries()

	params := sqlc.SaveJobPriorityParams{
		Now:      db.now(),
		ID:       int64(j.ID),
		Priority: int64(j.Priority),
	}

	err := queries.SaveJobPriority(ctx, params)
	if err != nil {
		return jobError(err, "saving job priority")
	}
	return nil
}

// SaveJobStorageInfo saves the job's Storage field.
// NOTE: this function does NOT update the job's `UpdatedAt` field. This is
// necessary for `cmd/shaman-checkout-id-setter` to do its work quietly.
func (db *DB) SaveJobStorageInfo(ctx context.Context, j *Job) error {
	queries := db.queries()

	params := sqlc.SaveJobStorageInfoParams{
		ID:                      int64(j.ID),
		StorageShamanCheckoutID: j.Storage.ShamanCheckoutID,
	}

	err := queries.SaveJobStorageInfo(ctx, params)
	if err != nil {
		return jobError(err, "saving job storage")
	}
	return nil
}

func (db *DB) FetchTask(ctx context.Context, taskUUID string) (*Task, error) {
	queries := db.queries()

	taskRow, err := queries.FetchTask(ctx, taskUUID)
	if err != nil {
		return nil, taskError(err, "fetching task %s", taskUUID)
	}

	return convertSqlTaskWithJobAndWorker(ctx, queries, taskRow.Task)
}

// TODO: remove this code, and let the code that calls into the persistence
// service fetch the job/worker explicitly when needed.
func convertSqlTaskWithJobAndWorker(
	ctx context.Context,
	queries *sqlc.Queries,
	task sqlc.Task,
) (*Task, error) {
	var (
		gormJob    Job
		gormWorker Worker
	)

	// Fetch & convert the Job.
	if task.JobID > 0 {
		sqlcJob, err := queries.FetchJobByID(ctx, task.JobID)
		if err != nil {
			return nil, jobError(err, "fetching job of task %s", task.UUID)
		}

		gormJob, err = convertSqlcJob(sqlcJob)
		if err != nil {
			return nil, jobError(err, "converting job of task %s", task.UUID)
		}
	}

	// Fetch & convert the Worker.
	if task.WorkerID.Valid && task.WorkerID.Int64 > 0 {
		sqlcWorker, err := queries.FetchWorkerUnconditionalByID(ctx, task.WorkerID.Int64)
		if err != nil {
			return nil, taskError(err, "fetching worker assigned to task %s", task.UUID)
		}
		gormWorker = convertSqlcWorker(sqlcWorker)
	}

	// Convert the Task.
	gormTask, err := convertSqlcTask(task, gormJob.UUID, gormWorker.UUID)
	if err != nil {
		return nil, err
	}

	// Put the Job & Worker into the Task.
	if gormJob.ID > 0 {
		gormTask.Job = &gormJob
		gormTask.JobUUID = gormJob.UUID
	}
	if gormWorker.ID > 0 {
		gormTask.Worker = &gormWorker
		gormTask.WorkerUUID = gormWorker.UUID
	}

	return gormTask, nil
}

// FetchTaskJobUUID fetches the job UUID of the given task.
func (db *DB) FetchTaskJobUUID(ctx context.Context, taskUUID string) (string, error) {
	queries := db.queries()

	jobUUID, err := queries.FetchTaskJobUUID(ctx, taskUUID)
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

	queries := db.queries()

	commandsJSON, err := json.Marshal(t.Commands)
	if err != nil {
		return fmt.Errorf("cannot convert commands to JSON: %w", err)
	}

	param := sqlc.UpdateTaskParams{
		UpdatedAt: db.now(),
		Name:      t.Name,
		Type:      t.Type,
		Priority:  int64(t.Priority),
		Status:    string(t.Status),
		Commands:  commandsJSON,
		Activity:  t.Activity,
		ID:        int64(t.ID),
	}
	if t.WorkerID != nil {
		param.WorkerID = sql.NullInt64{
			Int64: int64(*t.WorkerID),
			Valid: true,
		}
	} else if t.Worker != nil && t.Worker.ID > 0 {
		param.WorkerID = sql.NullInt64{
			Int64: int64(t.Worker.ID),
			Valid: true,
		}
	}

	if !t.LastTouchedAt.IsZero() {
		param.LastTouchedAt = sql.NullTime{
			Time:  t.LastTouchedAt,
			Valid: true,
		}
	}

	err = queries.UpdateTask(ctx, param)
	if err != nil {
		return taskError(err, "updating task")
	}
	return nil
}

func (db *DB) SaveTaskStatus(ctx context.Context, t *Task) error {
	queries := db.queries()

	err := queries.UpdateTaskStatus(ctx, sqlc.UpdateTaskStatusParams{
		UpdatedAt: db.now(),
		Status:    string(t.Status),
		ID:        int64(t.ID),
	})
	if err != nil {
		return taskError(err, "saving task status")
	}
	return nil
}

func (db *DB) SaveTaskActivity(ctx context.Context, t *Task) error {
	queries := db.queries()

	err := queries.UpdateTaskActivity(ctx, sqlc.UpdateTaskActivityParams{
		UpdatedAt: db.now(),
		Activity:  t.Activity,
		ID:        int64(t.ID),
	})
	if err != nil {
		return taskError(err, "saving task activity")
	}
	return nil
}

// TaskAssignToWorker assigns the given task to the given worker.
// This function is only used by unit tests. During normal operation, Flamenco
// uses the code in task_scheduler.go to assign tasks to workers.
func (db *DB) TaskAssignToWorker(ctx context.Context, t *Task, w *Worker) error {
	queries := db.queries()

	err := queries.TaskAssignToWorker(ctx, sqlc.TaskAssignToWorkerParams{
		UpdatedAt: db.now(),
		WorkerID: sql.NullInt64{
			Int64: int64(w.ID),
			Valid: true,
		},
		ID: int64(t.ID),
	})
	if err != nil {
		return taskError(err, "assigning task %s to worker %s", t.UUID, w.UUID)
	}

	// Update the task itself.
	t.Worker = w
	t.WorkerID = &w.ID

	return nil
}

func (db *DB) FetchTasksOfWorkerInStatus(ctx context.Context, worker *Worker, taskStatus api.TaskStatus) ([]*Task, error) {
	queries := db.queries()

	rows, err := queries.FetchTasksOfWorkerInStatus(ctx, sqlc.FetchTasksOfWorkerInStatusParams{
		WorkerID: sql.NullInt64{
			Int64: int64(worker.ID),
			Valid: true,
		},
		TaskStatus: string(taskStatus),
	})
	if err != nil {
		return nil, taskError(err, "finding tasks of worker %s in status %q", worker.UUID, taskStatus)
	}

	jobCache := make(map[uint]*Job)

	result := make([]*Task, len(rows))
	for i := range rows {
		jobUUID := rows[i].JobUUID.String
		gormTask, err := convertSqlcTask(rows[i].Task, jobUUID, worker.UUID)
		if err != nil {
			return nil, err
		}
		gormTask.Worker = worker
		gormTask.WorkerID = &worker.ID

		// Fetch the job, either from the cache or from the database. This is done
		// here because the task_state_machine functionality expects that task.Job
		// is set.
		// TODO: make that code fetch the job details it needs, rather than fetching
		// the entire job here.
		job := jobCache[gormTask.JobID]
		if job == nil {
			job, err = db.FetchJob(ctx, jobUUID)
			if err != nil {
				return nil, jobError(err, "finding job %s of task %s", jobUUID, gormTask.UUID)
			}
		}
		gormTask.Job = job

		result[i] = gormTask
	}
	return result, nil
}

func (db *DB) FetchTasksOfWorkerInStatusOfJob(ctx context.Context, worker *Worker, taskStatus api.TaskStatus, job *Job) ([]*Task, error) {
	queries := db.queries()

	rows, err := queries.FetchTasksOfWorkerInStatusOfJob(ctx, sqlc.FetchTasksOfWorkerInStatusOfJobParams{
		WorkerID: sql.NullInt64{
			Int64: int64(worker.ID),
			Valid: true,
		},
		JobID:      int64(job.ID),
		TaskStatus: string(taskStatus),
	})
	if err != nil {
		return nil, taskError(err, "finding tasks of worker %s in status %q and job %s", worker.UUID, taskStatus, job.UUID)
	}

	result := make([]*Task, len(rows))
	for i := range rows {
		gormTask, err := convertSqlcTask(rows[i].Task, job.UUID, worker.UUID)
		if err != nil {
			return nil, err
		}
		gormTask.Job = job
		gormTask.JobID = job.ID
		gormTask.Worker = worker
		gormTask.WorkerID = &worker.ID
		result[i] = gormTask
	}
	return result, nil
}

func (db *DB) JobHasTasksInStatus(ctx context.Context, job *Job, taskStatus api.TaskStatus) (bool, error) {
	queries := db.queries()

	count, err := queries.JobCountTasksInStatus(ctx, sqlc.JobCountTasksInStatusParams{
		JobID:      int64(job.ID),
		TaskStatus: string(taskStatus),
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
	queries := db.queries()

	results, err := queries.JobCountTaskStatuses(ctx, int64(job.ID))
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
func (db *DB) FetchTasksOfJob(ctx context.Context, job *Job) ([]*Task, error) {
	queries := db.queries()

	rows, err := queries.FetchTasksOfJob(ctx, int64(job.ID))
	if err != nil {
		return nil, taskError(err, "fetching tasks of job %s", job.UUID)
	}

	result := make([]*Task, len(rows))
	for i := range rows {
		gormTask, err := convertSqlcTask(rows[i].Task, job.UUID, rows[i].WorkerUUID.String)
		if err != nil {
			return nil, err
		}
		gormTask.Job = job
		result[i] = gormTask
	}
	return result, nil
}

// FetchTasksOfJobInStatus returns those tasks of the given job that have any of the given statuses.
func (db *DB) FetchTasksOfJobInStatus(ctx context.Context, job *Job, taskStatuses ...api.TaskStatus) ([]*Task, error) {
	queries := db.queries()

	rows, err := queries.FetchTasksOfJobInStatus(ctx, sqlc.FetchTasksOfJobInStatusParams{
		JobID:      int64(job.ID),
		TaskStatus: convertTaskStatuses(taskStatuses),
	})
	if err != nil {
		return nil, taskError(err, "fetching tasks of job %s in status %q", job.UUID, taskStatuses)
	}

	result := make([]*Task, len(rows))
	for i := range rows {
		gormTask, err := convertSqlcTask(rows[i].Task, job.UUID, rows[i].WorkerUUID.String)
		if err != nil {
			return nil, err
		}
		gormTask.Job = job
		result[i] = gormTask
	}
	return result, nil
}

// UpdateJobsTaskStatuses updates the status & activity of all tasks of `job`.
func (db *DB) UpdateJobsTaskStatuses(ctx context.Context, job *Job,
	taskStatus api.TaskStatus, activity string) error {

	if taskStatus == "" {
		return taskError(nil, "empty status not allowed")
	}

	queries := db.queries()

	err := queries.UpdateJobsTaskStatuses(ctx, sqlc.UpdateJobsTaskStatusesParams{
		UpdatedAt: db.now(),
		Status:    string(taskStatus),
		Activity:  activity,
		JobID:     int64(job.ID),
	})

	if err != nil {
		return taskError(err, "updating status of all tasks of job %s", job.UUID)
	}
	return nil
}

// UpdateJobsTaskStatusesConditional updates the status & activity of the tasks of `job`,
// limited to those tasks with status in `statusesToUpdate`.
func (db *DB) UpdateJobsTaskStatusesConditional(ctx context.Context, job *Job,
	statusesToUpdate []api.TaskStatus, taskStatus api.TaskStatus, activity string) error {

	if taskStatus == "" {
		return taskError(nil, "empty status not allowed")
	}

	queries := db.queries()

	err := queries.UpdateJobsTaskStatusesConditional(ctx, sqlc.UpdateJobsTaskStatusesConditionalParams{
		UpdatedAt:        db.now(),
		Status:           string(taskStatus),
		Activity:         activity,
		JobID:            int64(job.ID),
		StatusesToUpdate: convertTaskStatuses(statusesToUpdate),
	})

	if err != nil {
		return taskError(err, "updating status of all tasks in status %v of job %s", statusesToUpdate, job.UUID)
	}
	return nil
}

// TaskTouchedByWorker marks the task as 'touched' by a worker. This is used for timeout detection.
func (db *DB) TaskTouchedByWorker(ctx context.Context, t *Task) error {
	queries := db.queries()

	now := db.now()
	err := queries.TaskTouchedByWorker(ctx, sqlc.TaskTouchedByWorkerParams{
		UpdatedAt:     now,
		LastTouchedAt: now,
		ID:            int64(t.ID),
	})
	if err != nil {
		return taskError(err, "saving task 'last touched at'")
	}

	// Also update the given task, so that it's consistent with the database.
	t.LastTouchedAt = now.Time

	return nil
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
	queries := db.queries()

	err = queries.AddWorkerToTaskFailedList(ctx, sqlc.AddWorkerToTaskFailedListParams{
		CreatedAt: db.now().Time,
		TaskID:    int64(t.ID),
		WorkerID:  int64(w.ID),
	})
	if err != nil {
		return 0, err
	}

	numFailed64, err := queries.CountWorkersFailingTask(ctx, int64(t.ID))
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
	queries := db.queries()

	return queries.ClearFailureListOfTask(ctx, int64(t.ID))
}

// ClearFailureListOfJob en-mass, for all tasks of this job, clears the list of
// workers that failed those tasks.
func (db *DB) ClearFailureListOfJob(ctx context.Context, j *Job) error {
	queries := db.queries()

	return queries.ClearFailureListOfJob(ctx, int64(j.ID))
}

func (db *DB) FetchTaskFailureList(ctx context.Context, t *Task) ([]*Worker, error) {
	queries := db.queries()

	failureList, err := queries.FetchTaskFailureList(ctx, int64(t.ID))
	if err != nil {
		return nil, err
	}

	workers := make([]*Worker, len(failureList))
	for idx := range failureList {
		worker := convertSqlcWorker(failureList[idx].Worker)
		workers[idx] = &worker
	}
	return workers, nil
}

// convertSqlcJob converts a job from the SQLC-generated model to the model
// expected by the rest of the code. This is mostly in place to aid in the GORM
// to SQLC migration. It is intended that eventually the rest of the code will
// use the same SQLC-generated model.
func convertSqlcJob(job sqlc.Job) (Job, error) {
	dbJob := Job{
		Model: Model{
			ID:        uint(job.ID),
			CreatedAt: job.CreatedAt,
			UpdatedAt: job.UpdatedAt.Time,
		},
		UUID:              job.UUID,
		Name:              job.Name,
		JobType:           job.JobType,
		Priority:          int(job.Priority),
		Status:            api.JobStatus(job.Status),
		Activity:          job.Activity,
		DeleteRequestedAt: job.DeleteRequestedAt,
		Storage: JobStorageInfo{
			ShamanCheckoutID: job.StorageShamanCheckoutID,
		},
	}

	if err := json.Unmarshal(job.Settings, &dbJob.Settings); err != nil {
		return Job{}, jobError(err, fmt.Sprintf("job %s has invalid settings: %v", job.UUID, err))
	}

	if err := json.Unmarshal(job.Metadata, &dbJob.Metadata); err != nil {
		return Job{}, jobError(err, fmt.Sprintf("job %s has invalid metadata: %v", job.UUID, err))
	}

	if job.WorkerTagID.Valid {
		workerTagID := uint(job.WorkerTagID.Int64)
		dbJob.WorkerTagID = &workerTagID
	}

	return dbJob, nil
}

// convertSqlcTask converts a FetchTaskRow from the SQLC-generated model to the
// model expected by the rest of the code. This is mostly in place to aid in the
// GORM to SQLC migration. It is intended that eventually the rest of the code
// will use the same SQLC-generated model.
func convertSqlcTask(task sqlc.Task, jobUUID string, workerUUID string) (*Task, error) {
	dbTask := Task{
		Model: Model{
			ID:        uint(task.ID),
			CreatedAt: task.CreatedAt,
			UpdatedAt: task.UpdatedAt.Time,
		},

		UUID:          task.UUID,
		Name:          task.Name,
		Type:          task.Type,
		Priority:      int(task.Priority),
		Status:        api.TaskStatus(task.Status),
		LastTouchedAt: task.LastTouchedAt.Time,
		Activity:      task.Activity,

		JobID:      uint(task.JobID),
		JobUUID:    jobUUID,
		WorkerUUID: workerUUID,
	}

	// TODO: convert dependencies?

	if task.WorkerID.Valid {
		workerID := uint(task.WorkerID.Int64)
		dbTask.WorkerID = &workerID
	}

	if err := json.Unmarshal(task.Commands, &dbTask.Commands); err != nil {
		return nil, taskError(err, fmt.Sprintf("task %s of job %s has invalid commands: %v",
			task.UUID, jobUUID, err))
	}

	return &dbTask, nil
}

// convertTaskStatuses converts from []api.TaskStatus to []string for feeding to sqlc.
func convertTaskStatuses(taskStatuses []api.TaskStatus) []string {
	statusesAsStrings := make([]string, len(taskStatuses))
	for index := range taskStatuses {
		statusesAsStrings[index] = string(taskStatuses[index])
	}
	return statusesAsStrings
}

// convertJobStatuses converts from []api.JobStatus to []string for feeding to sqlc.
func convertJobStatuses(jobStatuses []api.JobStatus) []string {
	statusesAsStrings := make([]string, len(jobStatuses))
	for index := range jobStatuses {
		statusesAsStrings[index] = string(jobStatuses[index])
	}
	return statusesAsStrings
}
