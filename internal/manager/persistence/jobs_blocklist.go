package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"math"

	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
)

// JobBlockListEntry keeps track of which Worker is not allowed to run which task type on a given job.
type JobBlockListEntry = sqlc.FetchJobBlocklistRow

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
		CreatedAt: db.nowNullable().Time,
		JobID:     int64(job.ID),
		WorkerID:  int64(worker.ID),
		TaskType:  taskType,
	})
}

// FetchJobBlocklist fetches the blocklist for the given job.
// Workers are fetched too, and embedded in the returned list.
func (db *DB) FetchJobBlocklist(ctx context.Context, jobUUID string) ([]JobBlockListEntry, error) {
	queries := db.queries()

	rows, err := queries.FetchJobBlocklist(ctx, jobUUID)
	if err != nil {
		return nil, err
	}
	return rows, err
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
	queries := db.queries()

	var (
		workerUUIDs []string
		err         error
	)
	if job.WorkerTagID == nil {
		workerUUIDs, err = queries.WorkersLeftToRun(ctx, sqlc.WorkersLeftToRunParams{
			JobID:    int64(job.ID),
			TaskType: taskType,
		})
	} else {
		workerUUIDs, err = queries.WorkersLeftToRunWithWorkerTag(ctx,
			sqlc.WorkersLeftToRunWithWorkerTagParams{
				JobID:       int64(job.ID),
				TaskType:    taskType,
				WorkerTagID: int64(*job.WorkerTagID),
			})
	}
	if err != nil {
		return nil, err
	}

	// Construct a map of UUIDs.
	uuidMap := map[string]bool{}
	for _, uuid := range workerUUIDs {
		uuidMap[uuid] = true
	}

	return uuidMap, nil
}

// CountTaskFailuresOfWorker returns the number of task failures of this worker, on this particular job and task type.
func (db *DB) CountTaskFailuresOfWorker(ctx context.Context, job *Job, worker *Worker, taskType string) (int, error) {
	var numFailures int64

	queries := db.queries()
	numFailures, err := queries.CountTaskFailuresOfWorker(ctx, sqlc.CountTaskFailuresOfWorkerParams{
		JobID:    int64(job.ID),
		WorkerID: int64(worker.ID),
		TaskType: taskType,
	})

	if numFailures > math.MaxInt {
		panic("overflow error in number of failures")
	}

	return int(numFailures), err
}
