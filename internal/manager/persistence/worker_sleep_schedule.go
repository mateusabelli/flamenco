package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm/clause"
	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
)

// SleepSchedule belongs to a Worker, and determines when it's automatically
// sent to the 'asleep' and 'awake' states.
type SleepSchedule struct {
	Model

	WorkerID uint    `gorm:"default:0;unique;index"`
	Worker   *Worker `gorm:"foreignkey:WorkerID;references:ID;constraint:OnDelete:CASCADE"`

	IsActive bool `gorm:"default:false;index"`

	// Space-separated two-letter strings indicating days of week the schedule is
	// active ("mo", "tu", etc.). Empty means "every day".
	DaysOfWeek string    `gorm:"default:''"`
	StartTime  TimeOfDay `gorm:"default:''"`
	EndTime    TimeOfDay `gorm:"default:''"`

	NextCheck time.Time
}

// FetchWorkerSleepSchedule fetches the worker's sleep schedule.
// It does not fetch the worker itself. If you need that, call
// `FetchSleepScheduleWorker()` afterwards.
func (db *DB) FetchWorkerSleepSchedule(ctx context.Context, workerUUID string) (*SleepSchedule, error) {
	logger := log.With().Str("worker", workerUUID).Logger()
	logger.Trace().Msg("fetching worker sleep schedule")

	queries := db.queries()

	sqlcSched, err := queries.FetchWorkerSleepSchedule(ctx, workerUUID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	}

	return convertSqlcSleepSchedule(sqlcSched)
}

func (db *DB) SetWorkerSleepSchedule(ctx context.Context, workerUUID string, schedule *SleepSchedule) error {
	logger := log.With().Str("worker", workerUUID).Logger()
	logger.Trace().Msg("setting worker sleep schedule")

	worker, err := db.FetchWorker(ctx, workerUUID)
	if err != nil {
		return fmt.Errorf("fetching worker %q: %w", workerUUID, err)
	}
	schedule.WorkerID = worker.ID
	schedule.Worker = worker

	// Only store timestamps in UTC.
	if schedule.NextCheck.Location() != time.UTC {
		schedule.NextCheck = schedule.NextCheck.UTC()
	}

	tx := db.gormDB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "worker_id"}},
			UpdateAll: true,
		}).
		Create(&schedule)
	return tx.Error
}

func (db *DB) SetWorkerSleepScheduleNextCheck(ctx context.Context, schedule *SleepSchedule) error {
	// Only store timestamps in UTC.
	if schedule.NextCheck.Location() != time.UTC {
		schedule.NextCheck = schedule.NextCheck.UTC()
	}

	tx := db.gormDB.WithContext(ctx).
		Select("next_check").
		Updates(schedule)
	return tx.Error
}

// FetchSleepScheduleWorker sets the given schedule's `Worker` pointer.
func (db *DB) FetchSleepScheduleWorker(ctx context.Context, schedule *SleepSchedule) error {
	var worker Worker
	tx := db.gormDB.WithContext(ctx).Limit(1).Find(&worker, schedule.WorkerID)
	if tx.Error != nil {
		return workerError(tx.Error, "finding worker by their sleep schedule")
	}
	if worker.ID == 0 {
		// Worker was not found. It could be that the worker was soft-deleted, which
		// keeps the schedule around in the database.
		schedule.Worker = nil
		return ErrWorkerNotFound
	}
	schedule.Worker = &worker
	return nil
}

// FetchSleepSchedulesToCheck returns the sleep schedules that are due for a check.
func (db *DB) FetchSleepSchedulesToCheck(ctx context.Context) ([]*SleepSchedule, error) {
	now := db.gormDB.NowFunc()

	log.Debug().
		Str("timeout", now.String()).
		Msg("fetching sleep schedules that need checking")

	schedules := []*SleepSchedule{}
	tx := db.gormDB.WithContext(ctx).
		Model(&SleepSchedule{}).
		Where("is_active = ?", true).
		Where("next_check <= ? or next_check is NULL or next_check = ''", now).
		Scan(&schedules)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return schedules, nil
}

func convertSqlcSleepSchedule(sqlcSchedule sqlc.SleepSchedule) (*SleepSchedule, error) {
	schedule := SleepSchedule{
		Model: Model{
			ID:        uint(sqlcSchedule.ID),
			CreatedAt: sqlcSchedule.CreatedAt,
			UpdatedAt: sqlcSchedule.UpdatedAt.Time,
		},
		WorkerID:   uint(sqlcSchedule.WorkerID),
		IsActive:   sqlcSchedule.IsActive,
		DaysOfWeek: sqlcSchedule.DaysOfWeek,
	}

	err := schedule.StartTime.Scan(sqlcSchedule.StartTime)
	if err != nil {
		return nil, fmt.Errorf("parsing schedule start time %q: %w", sqlcSchedule.StartTime, err)
	}

	err = schedule.EndTime.Scan(sqlcSchedule.EndTime)
	if err != nil {
		return nil, fmt.Errorf("parsing schedule end time %q: %w", sqlcSchedule.EndTime, err)
	}

	return &schedule, nil
}
