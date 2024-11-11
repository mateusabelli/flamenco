package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
)

// SleepSchedule belongs to a Worker, and determines when it's automatically
// sent to the 'asleep' and 'awake' states.
type SleepSchedule struct {
	Model

	WorkerID uint
	Worker   *Worker

	IsActive bool

	// Space-separated two-letter strings indicating days of week the schedule is
	// active ("mo", "tu", etc.). Empty means "every day".
	DaysOfWeek string
	StartTime  TimeOfDay
	EndTime    TimeOfDay

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
	schedule.WorkerID = uint(worker.ID)
	schedule.Worker = worker

	// Only store timestamps in UTC.
	if schedule.NextCheck.Location() != time.UTC {
		schedule.NextCheck = schedule.NextCheck.UTC()
	}

	queries := db.queries()
	params := sqlc.SetWorkerSleepScheduleParams{
		CreatedAt:  db.now(),
		UpdatedAt:  db.nowNullable(),
		WorkerID:   int64(schedule.WorkerID),
		IsActive:   schedule.IsActive,
		DaysOfWeek: schedule.DaysOfWeek,
		StartTime:  schedule.StartTime.String(),
		EndTime:    schedule.EndTime.String(),
		NextCheck:  sql.NullTime{Time: schedule.NextCheck, Valid: !schedule.NextCheck.IsZero()},
	}

	id, err := queries.SetWorkerSleepSchedule(ctx, params)
	if err != nil {
		return fmt.Errorf("storing worker %q sleep schedule: %w", workerUUID, err)
	}
	schedule.ID = uint(id)
	return nil
}

func (db *DB) SetWorkerSleepScheduleNextCheck(ctx context.Context, schedule *SleepSchedule) error {
	// Only store timestamps in UTC.
	if schedule.NextCheck.Location() != time.UTC {
		schedule.NextCheck = schedule.NextCheck.UTC()
	}

	queries := db.queries()
	numAffected, err := queries.SetWorkerSleepScheduleNextCheck(
		ctx,
		sqlc.SetWorkerSleepScheduleNextCheckParams{
			ScheduleID: int64(schedule.ID),
			NextCheck:  sql.NullTime{Time: schedule.NextCheck, Valid: !schedule.NextCheck.IsZero()},
		})
	if err != nil {
		return fmt.Errorf("updating worker sleep schedule: %w", err)
	}
	if numAffected < 1 {
		return fmt.Errorf("could not find worker sleep schedule ID %d", schedule.ID)
	}
	return nil
}

// FetchSleepScheduleWorker sets the given schedule's `Worker` pointer.
func (db *DB) FetchSleepScheduleWorker(ctx context.Context, schedule *SleepSchedule) error {
	queries := db.queries()

	worker, err := queries.FetchWorkerByID(ctx, int64(schedule.WorkerID))
	if err != nil {
		schedule.Worker = nil
		return workerError(err, "finding worker by their sleep schedule")
	}

	schedule.Worker = &worker
	return nil
}

// FetchSleepSchedulesToCheck returns the sleep schedules that are due for a check.
func (db *DB) FetchSleepSchedulesToCheck(ctx context.Context) ([]*SleepSchedule, error) {
	now := db.nowNullable()

	log.Debug().
		Str("timeout", now.Time.String()).
		Msg("fetching sleep schedules that need checking")

	queries := db.queries()
	schedules, err := queries.FetchSleepSchedulesToCheck(ctx, now)
	if err != nil {
		return nil, err
	}

	gormSchedules := make([]*SleepSchedule, len(schedules))
	for index := range schedules {
		gormSched, err := convertSqlcSleepSchedule(schedules[index])
		if err != nil {
			return nil, err
		}
		gormSchedules[index] = gormSched
	}

	return gormSchedules, nil
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
