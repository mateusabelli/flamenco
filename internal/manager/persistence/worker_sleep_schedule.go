package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
)

// SleepSchedule belongs to a Worker, and determines when it's automatically
// sent to the 'asleep' and 'awake' states.
type SleepSchedule = sqlc.SleepSchedule

type SleepScheduleOwned struct {
	SleepSchedule SleepSchedule
	WorkerName    string
	WorkerUUID    string
}

// FetchWorkerSleepSchedule fetches the worker's sleep schedule.
func (db *DB) FetchWorkerSleepSchedule(ctx context.Context, workerUUID string) (*SleepSchedule, error) {
	logger := log.With().Str("worker", workerUUID).Logger()
	logger.Trace().Msg("fetching worker sleep schedule")

	queries := db.queries()

	schedule, err := queries.FetchWorkerSleepSchedule(ctx, workerUUID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	}
	return &schedule, nil
}

func (db *DB) SetWorkerSleepSchedule(ctx context.Context, workerUUID string, schedule *SleepSchedule) error {
	logger := log.With().Str("worker", workerUUID).Logger()
	logger.Trace().Msg("setting worker sleep schedule")

	worker, err := db.FetchWorker(ctx, workerUUID)
	if err != nil {
		return fmt.Errorf("fetching worker %q: %w", workerUUID, err)
	}
	schedule.WorkerID = worker.ID

	queries := db.queries()
	params := sqlc.SetWorkerSleepScheduleParams{
		CreatedAt:  db.now(),
		UpdatedAt:  db.nowNullable(),
		WorkerID:   schedule.WorkerID,
		IsActive:   schedule.IsActive,
		DaysOfWeek: schedule.DaysOfWeek,
		StartTime:  schedule.StartTime,
		EndTime:    schedule.EndTime,
		NextCheck:  nullTimeToUTC(schedule.NextCheck),
	}

	id, err := queries.SetWorkerSleepSchedule(ctx, params)
	if err != nil {
		return fmt.Errorf("storing worker %q sleep schedule: %w", workerUUID, err)
	}
	schedule.ID = id
	schedule.NextCheck = params.NextCheck
	schedule.CreatedAt = params.CreatedAt
	schedule.UpdatedAt = params.UpdatedAt
	return nil
}

func (db *DB) SetWorkerSleepScheduleNextCheck(ctx context.Context, schedule SleepSchedule) error {
	queries := db.queries()
	numAffected, err := queries.SetWorkerSleepScheduleNextCheck(
		ctx,
		sqlc.SetWorkerSleepScheduleNextCheckParams{
			ScheduleID: int64(schedule.ID),
			NextCheck:  nullTimeToUTC(schedule.NextCheck),
		})
	if err != nil {
		return fmt.Errorf("updating worker sleep schedule: %w", err)
	}
	if numAffected < 1 {
		return fmt.Errorf("could not find worker sleep schedule ID %d", schedule.ID)
	}
	return nil
}

// FetchSleepScheduleWorker returns the given schedule's associated Worker.
func (db *DB) FetchSleepScheduleWorker(ctx context.Context, schedule SleepSchedule) (*Worker, error) {
	queries := db.queries()

	worker, err := queries.FetchWorkerByID(ctx, schedule.WorkerID)
	if err != nil {
		return nil, workerError(err, "finding worker by their sleep schedule")
	}
	return &worker, nil
}

// FetchSleepSchedulesToCheck returns the sleep schedules that are due for a check, with their owning Worker.
func (db *DB) FetchSleepSchedulesToCheck(ctx context.Context) ([]SleepScheduleOwned, error) {
	now := db.nowNullable()

	log.Debug().
		Str("timeout", now.Time.String()).
		Msg("fetching sleep schedules that need checking")

	queries := db.queries()
	rows, err := queries.FetchSleepSchedulesToCheck(ctx, now)
	if err != nil {
		return nil, err
	}

	schedules := make([]SleepScheduleOwned, len(rows))
	for index, row := range rows {
		schedules[index].SleepSchedule = row.SleepSchedule
		schedules[index].WorkerName = row.WorkerName.String
		schedules[index].WorkerUUID = row.WorkerUUID.String
	}
	return schedules, nil
}
