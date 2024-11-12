package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
	"projects.blender.org/studio/flamenco/pkg/api"
)

type Worker = sqlc.Worker

func (db *DB) CreateWorker(ctx context.Context, w *Worker) error {
	queries := db.queries()

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

	workerID, err := queries.CreateWorker(ctx, params)
	if err != nil {
		return fmt.Errorf("creating new worker: %w", err)
	}

	w.ID = workerID
	w.CreatedAt = params.CreatedAt

	return nil
}

func (db *DB) FetchWorker(ctx context.Context, uuid string) (*Worker, error) {
	queries := db.queries()

	worker, err := queries.FetchWorker(ctx, uuid)
	if err != nil {
		return nil, workerError(err, "fetching worker %s", uuid)
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

	queries := db.queries()

	rowsAffected, err := queries.SoftDeleteWorker(ctx, sqlc.SoftDeleteWorkerParams{
		DeletedAt: db.nowNullable(),
		UUID:      uuid,
	})
	if err != nil {
		return workerError(err, "deleting worker")
	}
	if rowsAffected == 0 {
		return ErrWorkerNotFound
	}
	return nil
}

func (db *DB) FetchWorkers(ctx context.Context) ([]*Worker, error) {
	queries := db.queries()

	workers, err := queries.FetchWorkers(ctx)
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
	queries := db.queries()

	// Convert the WorkerID to a NullInt64. As task.worker_id can be NULL, this is
	// what sqlc expects us to pass in.
	workerID := sql.NullInt64{Int64: int64(worker.ID), Valid: true}

	row, err := queries.FetchWorkerTask(ctx, sqlc.FetchWorkerTaskParams{
		TaskStatusActive: api.TaskStatusActive,
		JobStatusActive:  api.JobStatusActive,
		WorkerID:         workerID,
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
	queries := db.queries()

	err := queries.SaveWorkerStatus(ctx, sqlc.SaveWorkerStatusParams{
		UpdatedAt:         db.nowNullable(),
		Status:            w.Status,
		StatusRequested:   w.StatusRequested,
		LazyStatusRequest: w.LazyStatusRequest,
		ID:                w.ID,
	})
	if err != nil {
		return fmt.Errorf("saving worker status: %w", err)
	}
	return nil
}

func (db *DB) SaveWorker(ctx context.Context, w *sqlc.Worker) error {
	if w.ID == 0 {
		panic("Do not use SaveWorker() to create a new Worker, use CreateWorker() instead")
	}

	queries := db.queries()

	err := queries.SaveWorker(ctx, sqlc.SaveWorkerParams{
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
	if err != nil {
		return fmt.Errorf("saving worker: %w", err)
	}
	return nil
}

// WorkerSeen marks the worker as 'seen' by this Manager. This is used for timeout detection.
func (db *DB) WorkerSeen(ctx context.Context, w *Worker) error {
	queries := db.queries()

	now := db.nowNullable()
	err := queries.WorkerSeen(ctx, sqlc.WorkerSeenParams{
		UpdatedAt:  now,
		LastSeenAt: now,
		ID:         int64(w.ID),
	})
	if err != nil {
		return workerError(err, "saving worker 'last seen at'")
	}
	return nil
}

// WorkerStatusCount is a mapping from job status to the number of jobs in that status.
type WorkerStatusCount map[api.WorkerStatus]int

func (db *DB) SummarizeWorkerStatuses(ctx context.Context) (WorkerStatusCount, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msg("database: summarizing worker statuses")

	queries := db.queries()

	rows, err := queries.SummarizeWorkerStatuses(ctx)
	if err != nil {
		return nil, workerError(err, "summarizing worker statuses")
	}

	statusCounts := make(WorkerStatusCount)
	for _, row := range rows {
		statusCounts[api.WorkerStatus(row.Status)] = int(row.StatusCount)
	}

	return statusCounts, nil
}
