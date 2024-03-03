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
	"gorm.io/gorm/clause"

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
	Priority int            `gorm:"type:smallint;default:50"`
	Status   api.TaskStatus `gorm:"type:varchar(16);default:''"`

	// Which worker is/was working on this.
	WorkerID      *uint
	Worker        *Worker   `gorm:"foreignkey:WorkerID;references:ID;constraint:OnDelete:SET NULL"`
	LastTouchedAt time.Time `gorm:"index"` // Should contain UTC timestamps.

	// Dependencies are tasks that need to be completed before this one can run.
	Dependencies []*Task `gorm:"many2many:task_dependencies;constraint:OnDelete:CASCADE"`

	Commands Commands `gorm:"type:jsonb"`
	Activity string   `gorm:"type:varchar(255);default:''"`
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
	return db.gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// TODO: separate conversion of struct types from storing things in the database.
		dbJob := Job{
			UUID:     authoredJob.JobID,
			Name:     authoredJob.Name,
			JobType:  authoredJob.JobType,
			Status:   authoredJob.Status,
			Priority: authoredJob.Priority,
			Settings: StringInterfaceMap(authoredJob.Settings),
			Metadata: StringStringMap(authoredJob.Metadata),
			Storage: JobStorageInfo{
				ShamanCheckoutID: authoredJob.Storage.ShamanCheckoutID,
			},
		}

		// Find and assign the worker tag.
		if authoredJob.WorkerTagUUID != "" {
			dbTag, err := fetchWorkerTag(tx, authoredJob.WorkerTagUUID)
			if err != nil {
				return err
			}
			dbJob.WorkerTagID = &dbTag.ID
			dbJob.WorkerTag = dbTag
		}

		if err := tx.Create(&dbJob).Error; err != nil {
			return jobError(err, "storing job")
		}

		return db.storeAuthoredJobTaks(ctx, tx, &dbJob, &authoredJob)
	})
}

// StoreAuthoredJobTaks is a low-level function that is only used for recreating an existing job's tasks.
// It stores `authoredJob`'s tasks, but attaches them to the already-persisted `job`.
func (db *DB) StoreAuthoredJobTaks(
	ctx context.Context,
	job *Job,
	authoredJob *job_compilers.AuthoredJob,
) error {
	tx := db.gormDB.WithContext(ctx)
	return db.storeAuthoredJobTaks(ctx, tx, job, authoredJob)
}

func (db *DB) storeAuthoredJobTaks(
	ctx context.Context,
	tx *gorm.DB,
	dbJob *Job,
	authoredJob *job_compilers.AuthoredJob,
) error {

	uuidToTask := make(map[string]*Task)
	for _, authoredTask := range authoredJob.Tasks {
		var commands []Command
		for _, authoredCommand := range authoredTask.Commands {
			commands = append(commands, Command{
				Name:       authoredCommand.Name,
				Parameters: StringInterfaceMap(authoredCommand.Parameters),
			})
		}

		dbTask := Task{
			Name:     authoredTask.Name,
			Type:     authoredTask.Type,
			UUID:     authoredTask.UUID,
			Job:      dbJob,
			Priority: authoredTask.Priority,
			Status:   api.TaskStatusQueued,
			Commands: commands,
			// dependencies are stored below.
		}
		if err := tx.Create(&dbTask).Error; err != nil {
			return taskError(err, "storing task: %v", err)
		}

		uuidToTask[authoredTask.UUID] = &dbTask
	}

	// Store the dependencies between tasks.
	for _, authoredTask := range authoredJob.Tasks {
		if len(authoredTask.Dependencies) == 0 {
			continue
		}

		dbTask, ok := uuidToTask[authoredTask.UUID]
		if !ok {
			return taskError(nil, "unable to find task %q in the database, even though it was just authored", authoredTask.UUID)
		}

		deps := make([]*Task, len(authoredTask.Dependencies))
		for i, t := range authoredTask.Dependencies {
			depTask, ok := uuidToTask[t.UUID]
			if !ok {
				return taskError(nil, "finding task with UUID %q; a task depends on a task that is not part of this job", t.UUID)
			}
			deps[i] = depTask
		}
		dependenciesbatchsize := 1000
		for j := 0; j < len(deps); j += dependenciesbatchsize {
			end := j + dependenciesbatchsize
			if end > len(deps) {
				end = len(deps)
			}
			currentDeps := deps[j:end]
			dbTask.Dependencies = currentDeps
			tx.Model(&dbTask).Where("UUID = ?", dbTask.UUID)
			subQuery := tx.Model(dbTask).Updates(Task{Dependencies: currentDeps})
			if subQuery.Error != nil {
				return taskError(subQuery.Error, "error with storing dependencies of task %q issue exists in dependencies %d to %d", authoredTask.UUID, j, end)
			}
		}
	}

	return nil
}

// FetchJob fetches a single job, without fetching its tasks.
func (db *DB) FetchJob(ctx context.Context, jobUUID string) (*Job, error) {
	queries, err := db.queries()
	if err != nil {
		return nil, err
	}

	sqlcJob, err := queries.FetchJob(ctx, jobUUID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrJobNotFound
	case err != nil:
		return nil, jobError(err, "fetching job")
	}

	return convertSqlcJob(sqlcJob)
}

// DeleteJob deletes a job from the database.
// The deletion cascades to its tasks and other job-related tables.
func (db *DB) DeleteJob(ctx context.Context, jobUUID string) error {
	// As a safety measure, refuse to delete jobs unless foreign key constraints are active.
	fkEnabled, err := db.areForeignKeysEnabled()
	if err != nil {
		return fmt.Errorf("checking whether foreign keys are enabled: %w", err)
	}
	if !fkEnabled {
		return ErrDeletingWithoutFK
	}

	queries, err := db.queries()
	if err != nil {
		return err
	}

	if err := queries.DeleteJob(ctx, jobUUID); err != nil {
		return jobError(err, "deleting job")
	}
	return nil
}

// RequestJobDeletion sets the job's "DeletionRequestedAt" field to "now".
func (db *DB) RequestJobDeletion(ctx context.Context, j *Job) error {
	queries, err := db.queries()
	if err != nil {
		return err
	}

	// Update the given job itself, so we don't have to re-fetch it from the database.
	j.DeleteRequestedAt.Time = db.gormDB.NowFunc()
	j.DeleteRequestedAt.Valid = true

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
	// In order to be able to report which jobs were affected, first fetch the
	// list of jobs, then update them.
	var jobs []*Job
	selectResult := db.gormDB.WithContext(ctx).
		Model(&Job{}).
		Select("uuid").
		Where("updated_at <= ?", lastUpdatedMax).
		Scan(&jobs)
	if selectResult.Error != nil {
		return nil, jobError(selectResult.Error, "fetching jobs by last-modified timestamp")
	}

	if len(jobs) == 0 {
		return nil, ErrJobNotFound
	}

	// Convert array of jobs to array of UUIDs.
	uuids := make([]string, len(jobs))
	for index := range jobs {
		uuids[index] = jobs[index].UUID
	}

	// Update the selected jobs.
	deleteRequestedAt := sql.NullTime{
		Time:  db.gormDB.NowFunc(),
		Valid: true,
	}
	tx := db.gormDB.WithContext(ctx).
		Model(Job{}).
		Where("uuid in ?", uuids).
		Updates(Job{DeleteRequestedAt: deleteRequestedAt})
	if tx.Error != nil {
		return nil, jobError(tx.Error, "queueing jobs for deletion")
	}

	return uuids, nil
}

func (db *DB) FetchJobsDeletionRequested(ctx context.Context) ([]string, error) {
	var jobs []*Job

	tx := db.gormDB.WithContext(ctx).
		Model(&Job{}).
		Select("UUID").
		Where("delete_requested_at is not NULL").
		Order("delete_requested_at").
		Scan(&jobs)

	if tx.Error != nil {
		return nil, jobError(tx.Error, "fetching jobs marked for deletion")
	}

	uuids := make([]string, len(jobs))
	for i := range jobs {
		uuids[i] = jobs[i].UUID
	}

	return uuids, nil
}

func (db *DB) FetchJobsInStatus(ctx context.Context, jobStatuses ...api.JobStatus) ([]*Job, error) {
	var jobs []*Job

	tx := db.gormDB.WithContext(ctx).
		Model(&Job{}).
		Where("status in ?", jobStatuses).
		Scan(&jobs)

	if tx.Error != nil {
		return nil, jobError(tx.Error, "fetching jobs in status %q", jobStatuses)
	}
	return jobs, nil
}

// SaveJobStatus saves the job's Status and Activity fields.
func (db *DB) SaveJobStatus(ctx context.Context, j *Job) error {
	tx := db.gormDB.WithContext(ctx).
		Model(j).
		Updates(Job{Status: j.Status, Activity: j.Activity})
	if tx.Error != nil {
		return jobError(tx.Error, "saving job status")
	}
	return nil
}

// SaveJobPriority saves the job's Priority field.
func (db *DB) SaveJobPriority(ctx context.Context, j *Job) error {
	tx := db.gormDB.WithContext(ctx).
		Model(j).
		Updates(Job{Priority: j.Priority})
	if tx.Error != nil {
		return jobError(tx.Error, "saving job priority")
	}
	return nil
}

// SaveJobStorageInfo saves the job's Storage field.
// NOTE: this function does NOT update the job's `UpdatedAt` field. This is
// necessary for `cmd/shaman-checkout-id-setter` to do its work quietly.
func (db *DB) SaveJobStorageInfo(ctx context.Context, j *Job) error {
	tx := db.gormDB.WithContext(ctx).
		Model(j).
		Omit("UpdatedAt").
		Updates(Job{Storage: j.Storage})
	if tx.Error != nil {
		return jobError(tx.Error, "saving job storage")
	}
	return nil
}

func (db *DB) FetchTask(ctx context.Context, taskUUID string) (*Task, error) {
	dbTask := Task{}
	tx := db.gormDB.WithContext(ctx).
		// Allow finding the Worker, even after it was deleted. Jobs and Tasks
		// don't have soft-deletion.
		Unscoped().
		Joins("Job").
		Joins("Worker").
		First(&dbTask, "tasks.uuid = ?", taskUUID)
	if tx.Error != nil {
		return nil, taskError(tx.Error, "fetching task")
	}
	return &dbTask, nil
}

func (db *DB) SaveTask(ctx context.Context, t *Task) error {
	tx := db.gormDB.WithContext(ctx).
		Omit("job").
		Omit("worker").
		Save(t)
	if tx.Error != nil {
		return taskError(tx.Error, "saving task")
	}
	return nil
}

func (db *DB) SaveTaskStatus(ctx context.Context, t *Task) error {
	tx := db.gormDB.WithContext(ctx).
		Select("Status").
		Save(t)
	if tx.Error != nil {
		return taskError(tx.Error, "saving task")
	}
	return nil
}

func (db *DB) SaveTaskActivity(ctx context.Context, t *Task) error {
	if err := db.gormDB.WithContext(ctx).
		Model(t).
		Select("Activity").
		Updates(Task{Activity: t.Activity}).Error; err != nil {
		return taskError(err, "saving task activity")
	}
	return nil
}

func (db *DB) TaskAssignToWorker(ctx context.Context, t *Task, w *Worker) error {
	tx := db.gormDB.WithContext(ctx).
		Model(t).
		Select("WorkerID").
		Updates(Task{WorkerID: &w.ID})
	if tx.Error != nil {
		return taskError(tx.Error, "assigning task %s to worker %s", t.UUID, w.UUID)
	}

	// Gorm updates t.WorkerID itself, but not t.Worker (even when it's added to
	// the Updates() call above).
	t.Worker = w

	return nil
}

func (db *DB) FetchTasksOfWorkerInStatus(ctx context.Context, worker *Worker, taskStatus api.TaskStatus) ([]*Task, error) {
	result := []*Task{}
	tx := db.gormDB.WithContext(ctx).
		Model(&Task{}).
		Joins("Job").
		Where("tasks.worker_id = ?", worker.ID).
		Where("tasks.status = ?", taskStatus).
		Scan(&result)
	if tx.Error != nil {
		return nil, taskError(tx.Error, "finding tasks of worker %s in status %q", worker.UUID, taskStatus)
	}
	return result, nil
}

func (db *DB) FetchTasksOfWorkerInStatusOfJob(ctx context.Context, worker *Worker, taskStatus api.TaskStatus, job *Job) ([]*Task, error) {
	result := []*Task{}
	tx := db.gormDB.WithContext(ctx).
		Model(&Task{}).
		Joins("Job").
		Where("tasks.worker_id = ?", worker.ID).
		Where("tasks.status = ?", taskStatus).
		Where("job.id = ?", job.ID).
		Scan(&result)
	if tx.Error != nil {
		return nil, taskError(tx.Error, "finding tasks of worker %s in status %q and job %s", worker.UUID, taskStatus, job.UUID)
	}
	return result, nil
}

func (db *DB) JobHasTasksInStatus(ctx context.Context, job *Job, taskStatus api.TaskStatus) (bool, error) {
	var numTasksInStatus int64
	tx := db.gormDB.WithContext(ctx).
		Model(&Task{}).
		Where("job_id", job.ID).
		Where("status", taskStatus).
		Count(&numTasksInStatus)
	if tx.Error != nil {
		return false, taskError(tx.Error, "counting tasks of job %s in status %q", job.UUID, taskStatus)
	}
	return numTasksInStatus > 0, nil
}

func (db *DB) CountTasksOfJobInStatus(
	ctx context.Context,
	job *Job,
	taskStatuses ...api.TaskStatus,
) (numInStatus, numTotal int, err error) {
	type Result struct {
		Status   api.TaskStatus
		NumTasks int
	}
	var results []Result

	tx := db.gormDB.WithContext(ctx).
		Model(&Task{}).
		Select("status, count(*) as num_tasks").
		Where("job_id", job.ID).
		Group("status").
		Scan(&results)

	if tx.Error != nil {
		return 0, 0, jobError(tx.Error, "count tasks of job %s in status %q", job.UUID, taskStatuses)
	}

	// Create lookup table for which statuses to count.
	countStatus := map[api.TaskStatus]bool{}
	for _, status := range taskStatuses {
		countStatus[status] = true
	}

	// Count the number of tasks per status.
	for _, result := range results {
		if countStatus[result.Status] {
			numInStatus += result.NumTasks
		}
		numTotal += result.NumTasks
	}

	return
}

// FetchTaskIDsOfJob returns all tasks of the given job.
func (db *DB) FetchTasksOfJob(ctx context.Context, job *Job) ([]*Task, error) {
	var tasks []*Task
	tx := db.gormDB.WithContext(ctx).
		Model(&Task{}).
		Where("job_id", job.ID).
		Scan(&tasks)
	if tx.Error != nil {
		return nil, taskError(tx.Error, "fetching tasks of job %s", job.UUID)
	}

	for i := range tasks {
		tasks[i].Job = job
	}

	return tasks, nil
}

// FetchTasksOfJobInStatus returns those tasks of the given job that have any of the given statuses.
func (db *DB) FetchTasksOfJobInStatus(ctx context.Context, job *Job, taskStatuses ...api.TaskStatus) ([]*Task, error) {
	var tasks []*Task
	tx := db.gormDB.WithContext(ctx).
		Model(&Task{}).
		Where("job_id", job.ID).
		Where("status in ?", taskStatuses).
		Scan(&tasks)
	if tx.Error != nil {
		return nil, taskError(tx.Error, "fetching tasks of job %s in status %q", job.UUID, taskStatuses)
	}

	for i := range tasks {
		tasks[i].Job = job
	}

	return tasks, nil
}

// UpdateJobsTaskStatuses updates the status & activity of all tasks of `job`.
func (db *DB) UpdateJobsTaskStatuses(ctx context.Context, job *Job,
	taskStatus api.TaskStatus, activity string) error {

	if taskStatus == "" {
		return taskError(nil, "empty status not allowed")
	}

	tx := db.gormDB.WithContext(ctx).
		Model(Task{}).
		Where("job_Id = ?", job.ID).
		Updates(Task{Status: taskStatus, Activity: activity})

	if tx.Error != nil {
		return taskError(tx.Error, "updating status of all tasks of job %s", job.UUID)
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

	tx := db.gormDB.WithContext(ctx).
		Model(Task{}).
		Where("job_Id = ?", job.ID).
		Where("status in ?", statusesToUpdate).
		Updates(Task{Status: taskStatus, Activity: activity})
	if tx.Error != nil {
		return taskError(tx.Error, "updating status of all tasks in status %v of job %s", statusesToUpdate, job.UUID)
	}
	return nil
}

// TaskTouchedByWorker marks the task as 'touched' by a worker. This is used for timeout detection.
func (db *DB) TaskTouchedByWorker(ctx context.Context, t *Task) error {
	tx := db.gormDB.WithContext(ctx).
		Model(t).
		Select("LastTouchedAt").
		Updates(Task{LastTouchedAt: db.gormDB.NowFunc()})
	if err := tx.Error; err != nil {
		return taskError(err, "saving task 'last touched at'")
	}
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
	entry := TaskFailure{
		Task:   t,
		Worker: w,
	}
	tx := db.gormDB.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&entry)
	if tx.Error != nil {
		return 0, tx.Error
	}

	var numFailed64 int64
	tx = db.gormDB.WithContext(ctx).Model(&TaskFailure{}).
		Where("task_id=?", t.ID).
		Count(&numFailed64)

	// Integer literals are of type `int`, so that's just a bit nicer to work with
	// than `int64`.
	if numFailed64 > math.MaxInt32 {
		log.Warn().Int64("numFailed", numFailed64).Msg("number of failed workers is crazy high, something is wrong here")
		return math.MaxInt32, tx.Error
	}
	return int(numFailed64), tx.Error
}

// ClearFailureListOfTask clears the list of workers that failed this task.
func (db *DB) ClearFailureListOfTask(ctx context.Context, t *Task) error {
	tx := db.gormDB.WithContext(ctx).
		Where("task_id = ?", t.ID).
		Delete(&TaskFailure{})
	return tx.Error
}

// ClearFailureListOfJob en-mass, for all tasks of this job, clears the list of
// workers that failed those tasks.
func (db *DB) ClearFailureListOfJob(ctx context.Context, j *Job) error {

	// SQLite doesn't support JOIN in DELETE queries, so use a sub-query instead.
	jobTasksQuery := db.gormDB.Model(&Task{}).
		Select("id").
		Where("job_id = ?", j.ID)

	tx := db.gormDB.WithContext(ctx).
		Where("task_id in (?)", jobTasksQuery).
		Delete(&TaskFailure{})
	return tx.Error
}

func (db *DB) FetchTaskFailureList(ctx context.Context, t *Task) ([]*Worker, error) {
	var workers []*Worker

	tx := db.gormDB.WithContext(ctx).
		Model(&Worker{}).
		Joins("inner join task_failures TF on TF.worker_id = workers.id").
		Where("TF.task_id = ?", t.ID).
		Scan(&workers)

	return workers, tx.Error
}

// convertSqlcJob converts a job from the SQLC-generated model to the model
// expected by the rest of the code. This is mostly in place to aid in the GORM
// to SQLC migration. It is intended that eventually the rest of the code will
// use the same SQLC-generated model.
func convertSqlcJob(job sqlc.Job) (*Job, error) {
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
		return nil, jobError(err, fmt.Sprintf("job %s has invalid settings: %v", job.UUID, err))
	}

	if err := json.Unmarshal(job.Metadata, &dbJob.Metadata); err != nil {
		return nil, jobError(err, fmt.Sprintf("job %s has invalid metadata: %v", job.UUID, err))
	}

	if job.WorkerTagID.Valid {
		workerTagID := uint(job.WorkerTagID.Int64)
		dbJob.WorkerTagID = &workerTagID
	}

	return &dbJob, nil
}
