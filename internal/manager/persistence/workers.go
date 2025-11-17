package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"errors"

	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
	"projects.blender.org/studio/flamenco/pkg/api"
)

type Worker = sqlc.Worker

func (db *DB) CreateWorker(ctx context.Context, w *Worker) error {
	now := db.nowNullable().Time
	params := sqlc.CreateWorkerParams{
		CreatedAt:          now,
		UUID:               w.UUID,
		Secret:             w.Secret,
		Name:               w.Name,
		Address:            w.Address,
		Platform:           w.Platform,
		Software:           w.Software,
		Status:             w.Status,
		LastSeenAt:         nullTimeToUTC(w.LastSeenAt),
		StatusRequested:    w.StatusRequested,
		LazyStatusRequest:  w.LazyStatusRequest,
		SupportedTaskTypes: w.SupportedTaskTypes,
		DeletedAt:          w.DeletedAt,
		CanRestart:         w.CanRestart,
	}

	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		workerID, err := q.CreateWorker(ctx, params)

		if err != nil {
			return err
		}

		w.ID = workerID
		w.CreatedAt = params.CreatedAt

		return nil
	})
}

func (db *DB) FetchWorker(ctx context.Context, uuid string) (*Worker, error) {
	var worker Worker

	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		worker, err = q.FetchWorker(ctx, uuid)
		return
	})

	if err != nil {
		return nil, workerError(err, "fetching worker %s", uuid)
	}
	return &worker, nil
}

func (db *DB) FetchWorkerByID(ctx context.Context, workerID int64) (*Worker, error) {
	var worker Worker

	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		worker, err = q.FetchWorkerByID(ctx, workerID)
		return
	})

	if err != nil {
		return nil, workerError(err, "fetching worker by ID %d", workerID)
	}
	return &worker, nil
}

func (db *DB) DeleteWorker(ctx context.Context, uuid string) error {
	// As a safety measure, refuse to delete unless foreign key constraints are active.
	fkEnabled, err := db.areForeignKeysEnabled(ctx)
	switch {
	case err != nil:
		return err
	case !fkEnabled:
		return ErrDeletingWithoutFK
	}

	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		rowsAffected, err := q.SoftDeleteWorker(ctx, sqlc.SoftDeleteWorkerParams{
			DeletedAt: db.nowNullable(),
			UUID:      uuid,
		})
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return ErrWorkerNotFound
		}
		return nil
	})
}

func (db *DB) FetchWorkers(ctx context.Context) ([]*Worker, error) {
	var workers []Worker

	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		workers, err = q.FetchWorkers(ctx)
		return
	})

	if err != nil {
		return nil, workerError(err, "fetching all workers")
	}

	workerPointers := make([]*Worker, len(workers))
	for idx := range workers {
		workerPointers[idx] = &workers[idx]
	}
	return workerPointers, nil
}

// FetchWorkerTask returns the most recent task assigned to the given Worker.
func (db *DB) FetchWorkerTask(ctx context.Context, worker *Worker) (*TaskJob, error) {
	// Convert the WorkerID to a NullInt64. As task.worker_id can be NULL, this is
	// what sqlc expects us to pass in.
	workerID := sql.NullInt64{Int64: int64(worker.ID), Valid: true}

	var row sqlc.FetchWorkerTaskRow
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		row, err = q.FetchWorkerTask(ctx, sqlc.FetchWorkerTaskParams{
			TaskStatusActive: api.TaskStatusActive,
			JobStatusActive:  api.JobStatusActive,
			WorkerID:         workerID,
		})
		return
	})

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, taskError(err, "fetching task assigned to Worker %s", worker.UUID)
	}

	taskJob := TaskJob{
		Task:     row.Task,
		JobUUID:  row.JobUUID,
		IsActive: row.IsActive,
	}
	return &taskJob, nil
}

func (db *DB) SaveWorkerStatus(ctx context.Context, w *Worker) error {
	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.SaveWorkerStatus(ctx, sqlc.SaveWorkerStatusParams{
			UpdatedAt:         db.nowNullable(),
			Status:            w.Status,
			StatusRequested:   w.StatusRequested,
			LazyStatusRequest: w.LazyStatusRequest,
			ID:                w.ID,
		})
	})

	return workerError(err, "saving worker status")
}

func (db *DB) SaveWorker(ctx context.Context, w *sqlc.Worker) error {
	if w.ID == 0 {
		panic("Do not use SaveWorker() to create a new Worker, use CreateWorker() instead")
	}

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.SaveWorker(ctx, sqlc.SaveWorkerParams{
			UpdatedAt:          db.nowNullable(),
			UUID:               w.UUID,
			Secret:             w.Secret,
			Name:               w.Name,
			Address:            w.Address,
			Platform:           w.Platform,
			Software:           w.Software,
			Status:             w.Status,
			LastSeenAt:         w.LastSeenAt,
			StatusRequested:    w.StatusRequested,
			LazyStatusRequest:  w.LazyStatusRequest,
			SupportedTaskTypes: w.SupportedTaskTypes,
			CanRestart:         w.CanRestart,
			ID:                 w.ID,
		})
	})

	return workerError(err, "saving worker status")
}

// WorkerSeen marks the worker as 'seen' by this Manager. This is used for timeout detection.
func (db *DB) WorkerSeen(ctx context.Context, w *Worker) error {
	now := db.nowNullable()

	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.WorkerSeen(ctx, sqlc.WorkerSeenParams{
			UpdatedAt:  now,
			LastSeenAt: now,
			ID:         int64(w.ID),
		})
	})

	return workerError(err, "saving worker 'last seen at'")
}

// WorkerStatusCount is a mapping from job status to the number of jobs in that status.
type WorkerStatusCount map[api.WorkerStatus]int

func (db *DB) SummarizeWorkerStatuses(ctx context.Context) (WorkerStatusCount, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msg("database: summarizing worker statuses")

	var rows []sqlc.SummarizeWorkerStatusesRow
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		rows, err = q.SummarizeWorkerStatuses(ctx)
		return
	})

	if err != nil {
		return nil, workerError(err, "summarizing worker statuses")
	}

	statusCounts := make(WorkerStatusCount)
	for _, row := range rows {
		statusCounts[api.WorkerStatus(row.Status)] = int(row.StatusCount)
	}

	return statusCounts, nil
}
