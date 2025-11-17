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
func (db *DB) AddWorkerToJobBlocklist(ctx context.Context, jobID int64, workerID int64, taskType string) error {
	if jobID == 0 {
		panic("Cannot add worker to job blocklist with zero job ID")
	}
	if workerID == 0 {
		panic("Cannot add worker to job blocklist with zero worker ID")
	}
	if taskType == "" {
		panic("Cannot add worker to job blocklist with empty task type")
	}

	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.AddWorkerToJobBlocklist(ctx, sqlc.AddWorkerToJobBlocklistParams{
			CreatedAt: db.nowNullable().Time,
			JobID:     jobID,
			WorkerID:  workerID,
			TaskType:  taskType,
		})
	})
}

// FetchJobBlocklist fetches the blocklist for the given job.
// Workers are fetched too, and embedded in the returned list.
func (db *DB) FetchJobBlocklist(ctx context.Context, jobUUID string) ([]JobBlockListEntry, error) {
	var rows []JobBlockListEntry

	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		rows, err = q.FetchJobBlocklist(ctx, jobUUID)
		return
	})

	return rows, err
}

// ClearJobBlocklist removes the entire blocklist of this job.
func (db *DB) ClearJobBlocklist(ctx context.Context, job *Job) error {
	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.ClearJobBlocklist(ctx, job.UUID)
	})
}

func (db *DB) RemoveFromJobBlocklist(ctx context.Context, jobUUID, workerUUID, taskType string) error {
	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.RemoveFromJobBlocklist(ctx, sqlc.RemoveFromJobBlocklistParams{
			JobUUID:    jobUUID,
			WorkerUUID: workerUUID,
			TaskType:   taskType,
		})
	})
}

// WorkersLeftToRun returns a set of worker UUIDs that can run tasks of the given type on the given job.
//
// NOTE: this does NOT consider the task failure list, which blocks individual
// workers from individual tasks. This is ONLY concerning the job blocklist.
func (db *DB) WorkersLeftToRun(ctx context.Context, job *Job, taskType string) (map[string]bool, error) {
	var workerUUIDs []string

	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		if job.WorkerTagID.Valid {
			workerUUIDs, err = q.WorkersLeftToRunWithWorkerTag(ctx,
				sqlc.WorkersLeftToRunWithWorkerTagParams{
					JobID:       job.ID,
					TaskType:    taskType,
					WorkerTagID: job.WorkerTagID.Int64,
				})
		} else {
			workerUUIDs, err = q.WorkersLeftToRun(ctx, sqlc.WorkersLeftToRunParams{
				JobID:    job.ID,
				TaskType: taskType,
			})
		}
		return
	})
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
func (db *DB) CountTaskFailuresOfWorker(ctx context.Context, jobUUID string, workerID int64, taskType string) (int, error) {
	var numFailures int64

	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		numFailures, err = q.CountTaskFailuresOfWorker(ctx, sqlc.CountTaskFailuresOfWorkerParams{
			JobUUID:  jobUUID,
			WorkerID: workerID,
			TaskType: taskType,
		})
		return
	})

	if numFailures > math.MaxInt {
		panic("overflow error in number of failures")
	}

	return int(numFailures), err
}
