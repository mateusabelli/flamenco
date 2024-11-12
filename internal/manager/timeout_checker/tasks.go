package timeout_checker

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

func (ttc *TimeoutChecker) checkTasks(ctx context.Context) {
	timeoutThreshold := ttc.clock.Now().UTC().Add(-ttc.taskTimeout)
	logger := log.With().
		Time("threshold", timeoutThreshold.Local()).
		Logger()
	logger.Trace().Msg("TimeoutChecker: finding active tasks that have not been touched since threshold")

	timeoutTaskInfo, err := ttc.persist.FetchTimedOutTasks(ctx, timeoutThreshold)
	if err != nil {
		log.Error().Err(err).Msg("TimeoutChecker: error fetching timed-out tasks from database")
		return
	}

	if len(timeoutTaskInfo) == 0 {
		logger.Trace().Msg("TimeoutChecker: no timed-out tasks")
		return
	}
	logger.Debug().
		Int("numTasks", len(timeoutTaskInfo)).
		Msg("TimeoutChecker: failing all active tasks that have not been touched since threshold")

	for _, taskInfo := range timeoutTaskInfo {
		ttc.timeoutTask(ctx, taskInfo)
	}
}

// timeoutTask marks a task as 'failed' due to a timeout.
func (ttc *TimeoutChecker) timeoutTask(ctx context.Context, taskInfo persistence.TimedOutTaskInfo) {
	task := taskInfo.Task
	workerIdent, logger := ttc.assignedWorker(taskInfo)

	task.Activity = fmt.Sprintf("Task timed out on worker %s", workerIdent)
	err := ttc.taskStateMachine.TaskStatusChange(ctx, &task, api.TaskStatusFailed)
	if err != nil {
		logger.Error().Err(err).Msg("TimeoutChecker: error saving timed-out task to database")
	}

	lastTouchedAt := "forever"
	if task.LastTouchedAt.Valid {
		lastTouchedAt = task.LastTouchedAt.Time.Format(time.RFC3339)
	}

	err = ttc.logStorage.WriteTimestamped(logger, taskInfo.JobUUID, task.UUID,
		fmt.Sprintf("Task timed out. It was assigned to worker %s, but untouched since %s",
			workerIdent, lastTouchedAt))
	if err != nil {
		logger.Error().Err(err).Msg("TimeoutChecker: error writing timeout info to the task log")
	}
}

// assignedWorker returns a description of the worker assigned to this task,
// and a logger configured for it.
func (ttc *TimeoutChecker) assignedWorker(taskInfo persistence.TimedOutTaskInfo) (string, zerolog.Logger) {
	logCtx := log.With().Str("task", taskInfo.Task.UUID)

	if taskInfo.WorkerUUID == "" {
		logger := logCtx.Logger()
		logger.Warn().Msg("TimeoutChecker: task timed out, but was not assigned to any worker")
		return "-unassigned-", logger
	}

	logCtx = logCtx.
		Str("worker", taskInfo.WorkerUUID).
		Str("workerName", taskInfo.WorkerName)
	logger := logCtx.Logger()
	logger.Warn().Msg("TimeoutChecker: task timed out")

	return fmt.Sprintf("%s (%s)", taskInfo.WorkerName, taskInfo.WorkerUUID), logger
}
