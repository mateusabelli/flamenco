package task_state_machine

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// RequeueActiveTasksOfWorker re-queues all active tasks (should be max one) of this worker.
//
// `reason`: a string that can be appended to text like "Task requeued because "
func (sm *StateMachine) RequeueActiveTasksOfWorker(
	ctx context.Context,
	worker *persistence.Worker,
	reason string,
) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Fetch the tasks to update.
	tasksJobs, err := sm.persist.FetchTasksOfWorkerInStatus(
		ctx, worker, api.TaskStatusActive)
	if err != nil {
		return err
	}

	// Run each task change through the task state machine.
	var lastErr error
	for _, taskJobWorker := range tasksJobs {
		lastErr = sm.returnTaskOfWorker(ctx, &taskJobWorker.Task, taskJobWorker.JobUUID, worker, reason)
	}

	return lastErr
}

// RequeueFailedTasksOfWorkerOfJob re-queues all failed tasks of this worker on this job.
//
// `reason`: a string that can be appended to text like "Task requeued because "
func (sm *StateMachine) RequeueFailedTasksOfWorkerOfJob(
	ctx context.Context,
	worker *persistence.Worker,
	jobUUID string,
	reason string,
) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Fetch the tasks to update.
	tasks, err := sm.persist.FetchTasksOfWorkerInStatusOfJob(
		ctx, worker, api.TaskStatusFailed, jobUUID)
	if err != nil {
		return err
	}

	// Run each task change through the task state machine.
	var lastErr error
	for _, task := range tasks {
		lastErr = sm.returnTaskOfWorker(ctx, task, jobUUID, worker, reason)
	}

	return lastErr
}

// returnTaskOfWorker returns the task to the task pool.
// This either re-queues the task for execution, or pauses it, depending on the current job status.
func (sm *StateMachine) returnTaskOfWorker(
	ctx context.Context,
	task *persistence.Task,
	jobUUID string,
	worker *persistence.Worker,
	reason string,
) error {
	logger := log.With().
		Str("worker", worker.UUID).
		Str("reason", reason).
		Logger()

	job, err := sm.persist.FetchJob(ctx, jobUUID)
	if err != nil {
		return fmt.Errorf("could not requeue task of worker %q: %w", worker.UUID, err)
	}

	// Depending on the job's status, a Worker returning its task to the pool should make it go to 'queued' or 'paused'.
	var targetTaskStatus api.TaskStatus
	switch job.Status {
	case api.JobStatusPauseRequested, api.JobStatusPaused:
		targetTaskStatus = api.TaskStatusPaused
	default:
		targetTaskStatus = api.TaskStatusQueued
	}

	logger.Info().
		Str("task", task.UUID).
		Str("newTaskStatus", string(targetTaskStatus)).
		Msg("returning task to pool")

		// Write to task activity that it got requeued because of worker sign-off.
	task.Activity = fmt.Sprintf("Task was %s by Manager because %s", targetTaskStatus, reason)
	if err := sm.persist.SaveTaskActivity(ctx, task); err != nil {
		logger.Warn().Err(err).
			Str("task", task.UUID).
			Str("reason", reason).
			Str("activity", task.Activity).
			Msg("error saving task activity to database")
	}

	if err := sm.taskStatusChange(ctx, task, targetTaskStatus); err != nil {
		logger.Warn().Err(err).
			Str("task", task.UUID).
			Str("reason", reason).
			Msg("error returning task to pool")
	}

	// The error is already logged by the log storage.
	_ = sm.logStorage.WriteTimestamped(logger, jobUUID, task.UUID, task.Activity)

	return err
}
