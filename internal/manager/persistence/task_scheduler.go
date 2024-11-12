package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
	"projects.blender.org/studio/flamenco/pkg/api"
)

var (
	// Note that active tasks are not schedulable, because they're already dunning on some worker.
	schedulableTaskStatuses = []api.TaskStatus{api.TaskStatusQueued, api.TaskStatusSoftFailed}
	schedulableJobStatuses  = []api.JobStatus{api.JobStatusActive, api.JobStatusQueued}
	// completedTaskStatuses   = []api.TaskStatus{api.TaskStatusCompleted}
)

// ScheduledTask contains a Task and some info about its job.
//
// This structure is returned from different points in the code below, and
// filled from different sqlc-generated structs. That's why it has to be an
// explicit struct here, rather than an alias for some sqlc struct.
type ScheduledTask struct {
	Task        Task
	JobUUID     string
	JobPriority int64
	JobType     string
}

// ScheduleTask finds a task to execute by the given worker.
// If no task is available, (nil, nil) is returned, as this is not an error situation.
// NOTE: this does not also fetch returnedTask.Worker, but returnedTask.WorkerID is set.
func (db *DB) ScheduleTask(ctx context.Context, w *Worker) (*ScheduledTask, error) {
	logger := log.With().Str("worker", w.UUID).Logger()
	logger.Trace().Msg("finding task for worker")

	// Run all queries in a single transaction.
	//
	// After this point, all queries should use this transaction. Otherwise SQLite
	// will deadlock, as it will make any other query wait until this transaction
	// is done.
	qtx, err := db.queriesWithTX()
	if err != nil {
		return nil, err
	}

	defer qtx.rollback()

	scheduledTask, err := db.scheduleTask(ctx, qtx.queries, w, logger)
	if err != nil {
		return nil, err
	}

	if scheduledTask == nil {
		// No task means no changes to the database.
		// It's fine to just roll back the transaction.
		return nil, nil
	}

	if err := qtx.commit(); err != nil {
		return nil, fmt.Errorf(
			"could not commit database transaction after scheduling task %s for worker %s: %w",
			scheduledTask.Task.UUID, w.UUID, err)
	}

	return scheduledTask, nil
}

func (db *DB) scheduleTask(ctx context.Context, queries *sqlc.Queries, w *Worker, logger zerolog.Logger) (*ScheduledTask, error) {
	if w.ID == 0 {
		panic("worker should be in database, but has zero ID")
	}
	workerID := sql.NullInt64{Int64: int64(w.ID), Valid: true}

	// If a task is alreay active & assigned to this worker, return just that.
	// Note that this task type could be blocklisted or no longer supported by the
	// Worker, but since it's active that is unlikely.
	{
		row, err := queries.FetchAssignedAndRunnableTaskOfWorker(
			ctx, sqlc.FetchAssignedAndRunnableTaskOfWorkerParams{
				ActiveTaskStatus:  api.TaskStatusActive,
				ActiveJobStatuses: schedulableJobStatuses,
				WorkerID:          workerID,
			})

		switch {
		case errors.Is(err, sql.ErrNoRows):
			// Fine, just means there was no task assigned yet.
		case err != nil:
			return nil, err
		case row.Task.ID > 0:
			// Task was previously assigned, just go for it again.
			scheduledTask := ScheduledTask{
				Task:        row.Task,
				JobUUID:     row.JobUUID,
				JobPriority: row.JobPriority,
				JobType:     row.JobType,
			}
			return &scheduledTask, nil
		}
	}

	scheduledTask, err := findTaskForWorker(ctx, queries, w)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Fine, just means there was no task assigned yet.
		return nil, nil
	case isDatabaseBusyError(err):
		logger.Trace().Err(err).Msg("database busy while finding task for worker")
		return nil, errDatabaseBusy
	case err != nil:
		logger.Error().Err(err).Msg("finding task for worker")
		return nil, fmt.Errorf("finding task for worker: %w", err)
	}

	// Assign the task to the worker.
	assignmentTimestamp := db.nowNullable()
	err = queries.AssignTaskToWorker(ctx, sqlc.AssignTaskToWorkerParams{
		WorkerID: workerID,
		TaskID:   scheduledTask.Task.ID,
		Now:      assignmentTimestamp,
	})

	switch {
	case isDatabaseBusyError(err):
		logger.Trace().Err(err).Msg("database busy while assigning task to worker")
		return nil, errDatabaseBusy
	case err != nil:
		logger.Warn().
			Str("taskID", scheduledTask.Task.UUID).
			Err(err).
			Msg("assigning task to worker")
		return nil, fmt.Errorf("assigning task to worker: %w", err)
	}

	// Make sure the returned task matches the database.
	scheduledTask.Task.WorkerID = workerID
	scheduledTask.Task.UpdatedAt = assignmentTimestamp

	logger.Info().
		Str("taskID", scheduledTask.Task.UUID).
		Msg("assigned task to worker")

	return scheduledTask, nil
}

func findTaskForWorker(
	ctx context.Context,
	queries *sqlc.Queries,
	w *Worker,
) (*ScheduledTask, error) {

	// Construct the list of worker tag IDs to check.
	tags, err := queries.FetchTagsOfWorker(ctx, w.UUID)
	if err != nil {
		return nil, err
	}
	workerTags := make([]sql.NullInt64, len(tags))
	for index, tag := range tags {
		workerTags[index] = sql.NullInt64{Int64: tag.ID, Valid: true}
	}

	row, err := queries.FindRunnableTask(ctx, sqlc.FindRunnableTaskParams{
		WorkerID:                int64(w.ID),
		SchedulableTaskStatuses: schedulableTaskStatuses,
		SchedulableJobStatuses:  schedulableJobStatuses,
		SupportedTaskTypes:      w.TaskTypes(),
		TaskStatusCompleted:     api.TaskStatusCompleted,
		WorkerTags:              workerTags,
	})
	if err != nil {
		return nil, err
	}
	if row.Task.ID == 0 {
		return nil, nil
	}

	scheduledTask := ScheduledTask{
		Task:        row.Task,
		JobUUID:     row.JobUUID,
		JobPriority: row.JobPriority,
		JobType:     row.JobType,
	}
	return &scheduledTask, nil
}
