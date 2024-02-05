package timeout_checker

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"projects.blender.org/studio/flamenco/internal/manager/eventbus"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/internal/manager/task_state_machine"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// Generate mock implementations of these interfaces.
//go:generate go run github.com/golang/mock/mockgen -destination mocks/interfaces_mock.gen.go -package mocks projects.blender.org/studio/flamenco/internal/manager/timeout_checker PersistenceService,TaskStateMachine,LogStorage,ChangeBroadcaster

type PersistenceService interface {
	FetchTimedOutTasks(ctx context.Context, untouchedSince time.Time) ([]*persistence.Task, error)
	FetchTimedOutWorkers(ctx context.Context, lastSeenBefore time.Time) ([]*persistence.Worker, error)
	SaveWorker(ctx context.Context, w *persistence.Worker) error
}

var _ PersistenceService = (*persistence.DB)(nil)

type TaskStateMachine interface {
	// TaskStatusChange gives a Task a new status, and handles the resulting status changes on the job.
	TaskStatusChange(ctx context.Context, task *persistence.Task, newStatus api.TaskStatus) error
	RequeueActiveTasksOfWorker(ctx context.Context, worker *persistence.Worker, reason string) error
}

var _ TaskStateMachine = (*task_state_machine.StateMachine)(nil)

// LogStorage is used to append timeout messages to task logs.
type LogStorage interface {
	WriteTimestamped(logger zerolog.Logger, jobID, taskID string, logText string) error
}

// TODO: Refactor the way worker status changes are handled, so that this
// service doens't need to broadcast its own worker updates.
type ChangeBroadcaster interface {
	BroadcastWorkerUpdate(workerUpdate api.EventWorkerUpdate)
}

// ChangeBroadcaster should be a subset of eventbus.Broker.
var _ ChangeBroadcaster = (*eventbus.Broker)(nil)
