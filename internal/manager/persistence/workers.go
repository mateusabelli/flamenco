package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
	"projects.blender.org/studio/flamenco/pkg/api"
)

type Worker struct {
	Model
	DeletedAt gorm.DeletedAt `gorm:"index"`

	UUID   string `gorm:"type:char(36);default:'';unique;index"`
	Secret string `gorm:"type:varchar(255);default:''"`
	Name   string `gorm:"type:varchar(64);default:''"`

	Address    string           `gorm:"type:varchar(39);default:'';index"` // 39 = max length of IPv6 address.
	Platform   string           `gorm:"type:varchar(16);default:''"`
	Software   string           `gorm:"type:varchar(32);default:''"`
	Status     api.WorkerStatus `gorm:"type:varchar(16);default:''"`
	LastSeenAt time.Time        `gorm:"index"` // Should contain UTC timestamps.
	CanRestart bool             `gorm:"type:smallint;default:false"`

	StatusRequested   api.WorkerStatus `gorm:"type:varchar(16);default:''"`
	LazyStatusRequest bool             `gorm:"type:smallint;default:false"`

	SupportedTaskTypes string `gorm:"type:varchar(255);default:''"` // comma-separated list of task types.

	Tags []*WorkerTag `gorm:"many2many:worker_tag_membership;constraint:OnDelete:CASCADE"`
}

func (w *Worker) Identifier() string {
	// Avoid a panic when worker.Identifier() is called on a nil pointer.
	if w == nil {
		return "-nil worker-"
	}
	return fmt.Sprintf("%s (%s)", w.Name, w.UUID)
}

// TaskTypes returns the worker's supported task types as list of strings.
func (w *Worker) TaskTypes() []string {
	return strings.Split(w.SupportedTaskTypes, ",")
}

// StatusChangeRequest stores a requested status change on the Worker.
// This just updates the Worker instance, but doesn't store the change in the
// database.
func (w *Worker) StatusChangeRequest(status api.WorkerStatus, isLazyRequest bool) {
	w.StatusRequested = status
	w.LazyStatusRequest = isLazyRequest
}

// StatusChangeClear clears the requested status change of the Worker.
// This just updates the Worker instance, but doesn't store the change in the
// database.
func (w *Worker) StatusChangeClear() {
	w.StatusRequested = ""
	w.LazyStatusRequest = false
}

func (db *DB) CreateWorker(ctx context.Context, w *Worker) error {
	queries, err := db.queries()
	if err != nil {
		return err
	}

	now := db.now().Time
	workerID, err := queries.CreateWorker(ctx, sqlc.CreateWorkerParams{
		CreatedAt: now,
		UUID:      w.UUID,
		Secret:    w.Secret,
		Name:      w.Name,
		Address:   w.Address,
		Platform:  w.Platform,
		Software:  w.Software,
		Status:    string(w.Status),
		LastSeenAt: sql.NullTime{
			Time:  w.LastSeenAt,
			Valid: !w.LastSeenAt.IsZero(),
		},
		StatusRequested:    string(w.StatusRequested),
		LazyStatusRequest:  w.LazyStatusRequest,
		SupportedTaskTypes: w.SupportedTaskTypes,
		DeletedAt:          sql.NullTime(w.DeletedAt),
		CanRestart:         w.CanRestart,
	})
	if err != nil {
		return fmt.Errorf("creating new worker: %w", err)
	}

	w.ID = uint(workerID)
	w.CreatedAt = now

	// TODO: remove the create-with-tags functionality to a higher-level function.
	// This code is just here to make this function work like the GORM code did.
	for _, tag := range w.Tags {
		err := queries.AddWorkerTagMembership(ctx, sqlc.AddWorkerTagMembershipParams{
			WorkerTagID: int64(tag.ID),
			WorkerID:    workerID,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) FetchWorker(ctx context.Context, uuid string) (*Worker, error) {
	queries, err := db.queries()
	if err != nil {
		return nil, err
	}

	worker, err := queries.FetchWorker(ctx, uuid)
	if err != nil {
		return nil, workerError(err, "fetching worker %s", uuid)
	}

	// TODO: remove this code, and let the caller fetch the tags when interested in them.
	workerTags, err := queries.FetchWorkerTags(ctx, uuid)
	if err != nil {
		return nil, workerTagError(err, "fetching tags of worker %s", uuid)
	}

	convertedWorker := convertSqlcWorker(worker)
	convertedWorker.Tags = make([]*WorkerTag, len(workerTags))
	for index := range workerTags {
		convertedTag := convertSqlcWorkerTag(workerTags[index])
		convertedWorker.Tags[index] = &convertedTag
	}

	return &convertedWorker, nil
}

func (db *DB) DeleteWorker(ctx context.Context, uuid string) error {
	// As a safety measure, refuse to delete unless foreign key constraints are active.
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

	rowsAffected, err := queries.SoftDeleteWorker(ctx, sqlc.SoftDeleteWorkerParams{
		DeletedAt: db.now(),
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
	queries, err := db.queries()
	if err != nil {
		return nil, err
	}

	workers, err := queries.FetchWorkers(ctx)
	if err != nil {
		return nil, workerError(err, "fetching all workers")
	}

	gormWorkers := make([]*Worker, len(workers))
	for idx := range workers {
		worker := convertSqlcWorker(workers[idx].Worker)
		gormWorkers[idx] = &worker
	}
	return gormWorkers, nil
}

// FetchWorkerTask returns the most recent task assigned to the given Worker.
func (db *DB) FetchWorkerTask(ctx context.Context, worker *Worker) (*Task, error) {
	queries, err := db.queries()
	if err != nil {
		return nil, err
	}

	// Convert the WorkerID to a NullInt64. As task.worker_id can be NULL, this is
	// what sqlc expects us to pass in.
	workerID := sql.NullInt64{Int64: int64(worker.ID), Valid: true}

	row, err := queries.FetchWorkerTask(ctx, sqlc.FetchWorkerTaskParams{
		TaskStatusActive: string(api.TaskStatusActive),
		JobStatusActive:  string(api.JobStatusActive),
		WorkerID:         workerID,
	})

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, taskError(err, "fetching task assigned to Worker %s", worker.UUID)
	}

	// Found a task!
	if row.Job.ID == 0 {
		panic(fmt.Sprintf("task found but with no job: %#v", row))
	}
	if row.Task.ID == 0 {
		panic(fmt.Sprintf("task found but with zero ID: %#v", row))
	}

	// Convert the task & job to gorm data types.
	gormTask, err := convertSqlcTask(row.Task, row.Job.UUID, worker.UUID)
	if err != nil {
		return nil, err
	}
	gormJob, err := convertSqlcJob(row.Job)
	if err != nil {
		return nil, err
	}
	gormTask.Job = &gormJob
	gormTask.Worker = worker

	return gormTask, nil
}

func (db *DB) SaveWorkerStatus(ctx context.Context, w *Worker) error {
	queries, err := db.queries()
	if err != nil {
		return err
	}

	err = queries.SaveWorkerStatus(ctx, sqlc.SaveWorkerStatusParams{
		UpdatedAt:         db.now(),
		Status:            string(w.Status),
		StatusRequested:   string(w.StatusRequested),
		LazyStatusRequest: w.LazyStatusRequest,
		ID:                int64(w.ID),
	})
	if err != nil {
		return fmt.Errorf("saving worker status: %w", err)
	}
	return nil
}

func (db *DB) SaveWorker(ctx context.Context, w *Worker) error {
	// TODO: remove this code, and just let the caller call CreateWorker() directly.
	if w.ID == 0 {
		return db.CreateWorker(ctx, w)
	}

	queries, err := db.queries()
	if err != nil {
		return err
	}

	err = queries.SaveWorker(ctx, sqlc.SaveWorkerParams{
		UpdatedAt:          db.now(),
		UUID:               w.UUID,
		Secret:             w.Secret,
		Name:               w.Name,
		Address:            w.Address,
		Platform:           w.Platform,
		Software:           w.Software,
		Status:             string(w.Status),
		LastSeenAt:         sql.NullTime{Time: w.LastSeenAt, Valid: !w.LastSeenAt.IsZero()},
		StatusRequested:    string(w.StatusRequested),
		LazyStatusRequest:  w.LazyStatusRequest,
		SupportedTaskTypes: w.SupportedTaskTypes,
		CanRestart:         w.CanRestart,
		ID:                 int64(w.ID),
	})
	if err != nil {
		return fmt.Errorf("saving worker: %w", err)
	}
	return nil
}

// WorkerSeen marks the worker as 'seen' by this Manager. This is used for timeout detection.
func (db *DB) WorkerSeen(ctx context.Context, w *Worker) error {
	queries, err := db.queries()
	if err != nil {
		return err
	}

	now := db.now()
	err = queries.WorkerSeen(ctx, sqlc.WorkerSeenParams{
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

	queries, err := db.queries()
	if err != nil {
		return nil, err
	}

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

// convertSqlcWorker converts a worker from the SQLC-generated model to the model
// expected by the rest of the code. This is mostly in place to aid in the GORM
// to SQLC migration. It is intended that eventually the rest of the code will
// use the same SQLC-generated model.
func convertSqlcWorker(worker sqlc.Worker) Worker {
	return Worker{
		Model: Model{
			ID:        uint(worker.ID),
			CreatedAt: worker.CreatedAt,
			UpdatedAt: worker.UpdatedAt.Time,
		},
		DeletedAt: gorm.DeletedAt(worker.DeletedAt),

		UUID:               worker.UUID,
		Secret:             worker.Secret,
		Name:               worker.Name,
		Address:            worker.Address,
		Platform:           worker.Platform,
		Software:           worker.Software,
		Status:             api.WorkerStatus(worker.Status),
		LastSeenAt:         worker.LastSeenAt.Time,
		CanRestart:         worker.CanRestart,
		StatusRequested:    api.WorkerStatus(worker.StatusRequested),
		LazyStatusRequest:  worker.LazyStatusRequest,
		SupportedTaskTypes: worker.SupportedTaskTypes,
	}
}

// convertSqlcWorkerTag converts a worker tag from the SQLC-generated model to
// the model expected by the rest of the code. This is mostly in place to aid in
// the GORM to SQLC migration. It is intended that eventually the rest of the
// code will use the same SQLC-generated model.
func convertSqlcWorkerTag(tag sqlc.WorkerTag) WorkerTag {
	return WorkerTag{
		Model: Model{
			ID:        uint(tag.ID),
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt.Time,
		},
		UUID:        tag.UUID,
		Name:        tag.Name,
		Description: tag.Description,
	}
}
