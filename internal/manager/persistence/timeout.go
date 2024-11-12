package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"time"

	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// This file contains functions for dealing with task/worker timeouts. Not database timeouts.

// workerStatusNoTimeout contains the worker statuses that are exempt from
// timeout checking. A worker in any other status will be subject to the timeout
// check.
var workerStatusNoTimeout = []api.WorkerStatus{
	api.WorkerStatusError,
	api.WorkerStatusOffline,
}

type TimedOutTaskInfo = sqlc.FetchTimedOutTasksRow

// FetchTimedOutTasks returns a slice of tasks that have timed out.
//
// In order to time out, a task must be in status `active` and not touched by a
// Worker since `untouchedSince`.
func (db *DB) FetchTimedOutTasks(ctx context.Context, untouchedSince time.Time) ([]TimedOutTaskInfo, error) {
	queries := db.queries()

	timedOut, err := queries.FetchTimedOutTasks(ctx, sqlc.FetchTimedOutTasksParams{
		TaskStatus:     api.TaskStatusActive,
		UntouchedSince: sql.NullTime{Time: untouchedSince, Valid: true},
	})

	if err != nil {
		return nil, taskError(err, "finding timed out tasks (untouched since %s)", untouchedSince.String())
	}
	return timedOut, nil
}

func (db *DB) FetchTimedOutWorkers(ctx context.Context, lastSeenBefore time.Time) ([]*Worker, error) {
	queries := db.queries()

	sqlcWorkers, err := queries.FetchTimedOutWorkers(ctx, sqlc.FetchTimedOutWorkersParams{
		WorkerStatusesNoTimeout: workerStatusNoTimeout,
		LastSeenBefore: sql.NullTime{
			Time:  lastSeenBefore.UTC(),
			Valid: true},
	})
	if err != nil {
		return nil, workerError(err, "finding timed out workers (last seen before %s)", lastSeenBefore.String())
	}

	result := make([]*Worker, len(sqlcWorkers))
	for index := range sqlcWorkers {
		result[index] = &sqlcWorkers[index]
	}
	return result, nil
}
