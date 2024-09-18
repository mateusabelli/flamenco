package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"math"
	"time"

	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
)

// JobBlock keeps track of which Worker is not allowed to run which task type on which job.
type JobBlock struct {
	// Don't include the standard Gorm UpdatedAt or DeletedAt fields, as they're useless here.
	// Entries will never be updated, and should never be soft-deleted but just purged from existence.
	ID        uint
	CreatedAt time.Time

	JobID uint `gorm:"default:0;uniqueIndex:job_worker_tasktype"`
	Job   *Job `gorm:"foreignkey:JobID;references:ID;constraint:OnDelete:CASCADE"`

	WorkerID uint    `gorm:"default:0;uniqueIndex:job_worker_tasktype"`
	Worker   *Worker `gorm:"foreignkey:WorkerID;references:ID;constraint:OnDelete:CASCADE"`

	TaskType string `gorm:"uniqueIndex:job_worker_tasktype"`
}

// AddWorkerToJobBlocklist prevents this Worker of getting any task, of this type, on this job, from the task scheduler.
func (db *DB) AddWorkerToJobBlocklist(ctx context.Context, job *Job, worker *Worker, taskType string) error {
	if job.ID == 0 {
		panic("Cannot add worker to job blocklist with zero job ID")
	}
	if worker.ID == 0 {
		panic("Cannot add worker to job blocklist with zero worker ID")
	}
	if taskType == "" {
		panic("Cannot add worker to job blocklist with empty task type")
	}

	queries := db.queries()

	return queries.AddWorkerToJobBlocklist(ctx, sqlc.AddWorkerToJobBlocklistParams{
		CreatedAt: db.now().Time,
		JobID:     int64(job.ID),
		WorkerID:  int64(worker.ID),
		TaskType:  taskType,
	})
}

// FetchJobBlocklist fetches the blocklist for the given job.
// Workers are fetched too, and embedded in the returned list.
func (db *DB) FetchJobBlocklist(ctx context.Context, jobUUID string) ([]JobBlock, error) {
	queries := db.queries()

	rows, err := queries.FetchJobBlocklist(ctx, jobUUID)
	if err != nil {
		return nil, err
	}

	entries := make([]JobBlock, len(rows))
	for idx, row := range rows {
		entries[idx].ID = uint(row.JobBlock.ID)
		entries[idx].CreatedAt = row.JobBlock.CreatedAt
		entries[idx].TaskType = row.JobBlock.TaskType
		entries[idx].JobID = uint(row.JobBlock.JobID)
		entries[idx].WorkerID = uint(row.JobBlock.WorkerID)

		worker := convertSqlcWorker(row.Worker)
		entries[idx].Worker = &worker
	}

	return entries, nil
}

// ClearJobBlocklist removes the entire blocklist of this job.
func (db *DB) ClearJobBlocklist(ctx context.Context, job *Job) error {
	queries := db.queries()
	return queries.ClearJobBlocklist(ctx, job.UUID)
}

func (db *DB) RemoveFromJobBlocklist(ctx context.Context, jobUUID, workerUUID, taskType string) error {
	queries := db.queries()
	return queries.RemoveFromJobBlocklist(ctx, sqlc.RemoveFromJobBlocklistParams{
		JobUUID:    jobUUID,
		WorkerUUID: workerUUID,
		TaskType:   taskType,
	})
}

// WorkersLeftToRun returns a set of worker UUIDs that can run tasks of the given type on the given job.
//
// NOTE: this does NOT consider the task failure list, which blocks individual
// workers from individual tasks. This is ONLY concerning the job blocklist.
func (db *DB) WorkersLeftToRun(ctx context.Context, job *Job, taskType string) (map[string]bool, error) {
	// Find the IDs of the workers blocked on this job + tasktype combo.
	blockedWorkers := db.gormDB.
		Table("workers as blocked_workers").
		Select("blocked_workers.id").
		Joins("inner join job_blocks JB on blocked_workers.id = JB.worker_id").
		Where("JB.job_id = ?", job.ID).
		Where("JB.task_type = ?", taskType)

	query := db.gormDB.WithContext(ctx).
		Model(&Worker{}).
		Select("uuid").
		Where("id not in (?)", blockedWorkers)

	if job.WorkerTagID == nil {
		// Count all workers, so no extra restrictions are necessary.
	} else {
		// Only count workers in the job's tag.
		jobTag := db.gormDB.
			Table("worker_tag_membership").
			Select("worker_id").
			Where("worker_tag_id = ?", *job.WorkerTagID)
		query = query.
			Where("id in (?)", jobTag)
	}

	// Find the workers NOT blocked.
	workers := []*Worker{}
	tx := query.Scan(&workers)
	if tx.Error != nil {
		return nil, tx.Error
	}

	// From the list of workers, construct the map of UUIDs.
	uuidMap := map[string]bool{}
	for _, worker := range workers {
		uuidMap[worker.UUID] = true
	}

	return uuidMap, nil
}

// CountTaskFailuresOfWorker returns the number of task failures of this worker, on this particular job and task type.
func (db *DB) CountTaskFailuresOfWorker(ctx context.Context, job *Job, worker *Worker, taskType string) (int, error) {
	var numFailures int64

	tx := db.gormDB.WithContext(ctx).
		Model(&TaskFailure{}).
		Joins("inner join tasks T on task_failures.task_id = T.id").
		Where("task_failures.worker_id = ?", worker.ID).
		Where("T.job_id = ?", job.ID).
		Where("T.type = ?", taskType).
		Count(&numFailures)

	if numFailures > math.MaxInt {
		panic("overflow error in number of failures")
	}

	return int(numFailures), tx.Error
}
