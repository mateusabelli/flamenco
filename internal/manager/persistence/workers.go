package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
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
	if err := db.gormDB.WithContext(ctx).Create(w).Error; err != nil {
		return fmt.Errorf("creating new worker: %w", err)
	}
	return nil
}

func (db *DB) FetchWorker(ctx context.Context, uuid string) (*Worker, error) {
	w := Worker{}
	tx := db.gormDB.WithContext(ctx).
		Preload("Tags").
		Find(&w, "uuid = ?", uuid).
		Limit(1)
	if tx.Error != nil {
		return nil, workerError(tx.Error, "fetching worker")
	}
	if w.ID == 0 {
		return nil, ErrWorkerNotFound
	}
	return &w, nil
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

	tx := db.gormDB.WithContext(ctx).
		Where("uuid = ?", uuid).
		Delete(&Worker{})
	if tx.Error != nil {
		return workerError(tx.Error, "deleting worker")
	}
	if tx.RowsAffected == 0 {
		return ErrWorkerNotFound
	}
	return nil
}

func (db *DB) FetchWorkers(ctx context.Context) ([]*Worker, error) {
	workers := make([]*Worker, 0)
	tx := db.gormDB.WithContext(ctx).Model(&Worker{}).Scan(&workers)
	if tx.Error != nil {
		return nil, workerError(tx.Error, "fetching all workers")
	}
	return workers, nil
}

// FetchWorkerTask returns the most recent task assigned to the given Worker.
func (db *DB) FetchWorkerTask(ctx context.Context, worker *Worker) (*Task, error) {
	task := Task{}

	// See if there is a task assigned to this worker in the same way that the
	// task scheduler does.
	query := db.gormDB.WithContext(ctx)
	query = taskAssignedAndRunnableQuery(query, worker)
	tx := query.
		Order("tasks.updated_at").
		Preload("Job").
		Find(&task)
	if tx.Error != nil {
		return nil, taskError(tx.Error, "fetching task assigned to Worker %s", worker.UUID)
	}
	if task.ID != 0 {
		// Found a task!
		return &task, nil
	}

	// If not found, just find the last-modified task associated with this Worker.
	tx = db.gormDB.WithContext(ctx).
		Where("worker_id = ?", worker.ID).
		Order("tasks.updated_at DESC").
		Preload("Job").
		Find(&task)
	if tx.Error != nil {
		return nil, taskError(tx.Error, "fetching task assigned to Worker %s", worker.UUID)
	}
	if task.ID == 0 {
		return nil, nil
	}

	return &task, nil
}

func (db *DB) SaveWorkerStatus(ctx context.Context, w *Worker) error {
	err := db.gormDB.WithContext(ctx).
		Model(w).
		Select("status", "status_requested", "lazy_status_request").
		Updates(Worker{
			Status:            w.Status,
			StatusRequested:   w.StatusRequested,
			LazyStatusRequest: w.LazyStatusRequest,
		}).Error
	if err != nil {
		return fmt.Errorf("saving worker: %w", err)
	}
	return nil
}

func (db *DB) SaveWorker(ctx context.Context, w *Worker) error {
	if err := db.gormDB.WithContext(ctx).Save(w).Error; err != nil {
		return fmt.Errorf("saving worker: %w", err)
	}
	return nil
}

// WorkerSeen marks the worker as 'seen' by this Manager. This is used for timeout detection.
func (db *DB) WorkerSeen(ctx context.Context, w *Worker) error {
	tx := db.gormDB.WithContext(ctx).
		Model(w).
		Updates(Worker{LastSeenAt: db.gormDB.NowFunc()})
	if err := tx.Error; err != nil {
		return workerError(err, "saving worker 'last seen at'")
	}
	return nil
}

// WorkerStatusCount is a mapping from job status to the number of jobs in that status.
type WorkerStatusCount map[api.WorkerStatus]int

func (db *DB) SummarizeWorkerStatuses(ctx context.Context) (WorkerStatusCount, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msg("database: summarizing worker statuses")

	// Query the database using a data structure that's easy to handle in GORM.
	type queryResult struct {
		Status      api.WorkerStatus
		StatusCount int
	}
	result := []*queryResult{}
	tx := db.gormDB.WithContext(ctx).Model(&Worker{}).
		Select("status as Status", "count(id) as StatusCount").
		Group("status").
		Scan(&result)
	if tx.Error != nil {
		return nil, workerError(tx.Error, "summarizing worker statuses")
	}

	// Convert the array-of-structs to a map that's easier to handle by the caller.
	statusCounts := make(WorkerStatusCount)
	for _, singleStatusCount := range result {
		statusCounts[singleStatusCount.Status] = singleStatusCount.StatusCount
	}

	return statusCounts, nil
}
