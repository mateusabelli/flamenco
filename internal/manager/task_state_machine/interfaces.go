package task_state_machine

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"

	"github.com/rs/zerolog"
	"projects.blender.org/studio/flamenco/internal/manager/eventbus"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/internal/manager/task_logs"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// Generate mock implementations of these interfaces.
//go:generate go run github.com/golang/mock/mockgen -destination mocks/interfaces_mock.gen.go -package mocks projects.blender.org/studio/flamenco/internal/manager/task_state_machine PersistenceService,ChangeBroadcaster,LogStorage

type PersistenceService interface {
	SaveTask(ctx context.Context, task *persistence.Task) error
	SaveTaskStatus(ctx context.Context, t *persistence.Task) error
	SaveTaskActivity(ctx context.Context, t *persistence.Task) error
	SaveTaskStepsCompleted(ctx context.Context, jobID, taskID int64, stepsCompleted int64) error
	SaveJobStatus(ctx context.Context, j *persistence.Job) error

	JobHasTasksInStatus(ctx context.Context, job *persistence.Job, taskStatus api.TaskStatus) (bool, error)
	CountTasksOfJobInStatus(ctx context.Context, job *persistence.Job, taskStatuses ...api.TaskStatus) (numInStatus, numTotal int, err error)

	// UpdateJobsTaskStatuses updates the status & activity of the tasks of `job`.
	UpdateJobsTaskStatuses(ctx context.Context, job *persistence.Job,
		taskStatus api.TaskStatus, activity string) error

	// UpdateJobsTaskStatusesConditional updates the status & activity of the tasks of `job`,
	// limited to those tasks with status in `statusesToUpdate`.
	UpdateJobsTaskStatusesConditional(ctx context.Context, job *persistence.Job,
		statusesToUpdate []api.TaskStatus, taskStatus api.TaskStatus, activity string) error
	UpdateJobsTaskStepCounts(ctx context.Context, jobID int64) error

	FetchJob(ctx context.Context, jobUUID string) (*persistence.Job, error)
	FetchJobByID(ctx context.Context, jobID int64) (*persistence.Job, error)
	FetchJobsInStatus(ctx context.Context, jobStatuses ...api.JobStatus) ([]*persistence.Job, error)
	FetchTasksOfWorkerInStatus(context.Context, *persistence.Worker, api.TaskStatus) ([]persistence.TaskJob, error)
	FetchTasksOfWorkerInStatusOfJob(ctx context.Context, worker *persistence.Worker, status api.TaskStatus, jobUUID string) ([]*persistence.Task, error)
}

// PersistenceService should be a subset of persistence.DB
var _ PersistenceService = (*persistence.DB)(nil)

type ChangeBroadcaster interface {
	// BroadcastJobUpdate sends the job update to SocketIO clients.
	BroadcastJobUpdate(jobUpdate api.EventJobUpdate)

	// BroadcastTaskUpdate sends the task update to SocketIO clients.
	BroadcastTaskUpdate(jobUpdate api.EventTaskUpdate)
}

// ChangeBroadcaster should be a subset of eventbus.Broker
var _ ChangeBroadcaster = (*eventbus.Broker)(nil)

// LogStorage writes to task logs.
type LogStorage interface {
	WriteTimestamped(logger zerolog.Logger, jobID, taskID string, logText string) error
}

var _ LogStorage = (*task_logs.Storage)(nil)
