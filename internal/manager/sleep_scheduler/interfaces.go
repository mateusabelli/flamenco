package sleep_scheduler

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"

	"projects.blender.org/studio/flamenco/internal/manager/eventbus"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// Generate mock implementations of these interfaces.
//go:generate go run github.com/golang/mock/mockgen -destination mocks/interfaces_mock.gen.go -package mocks projects.blender.org/studio/flamenco/internal/manager/sleep_scheduler PersistenceService,ChangeBroadcaster

type PersistenceService interface {
	FetchWorkerSleepSchedule(ctx context.Context, workerUUID string) (*persistence.SleepSchedule, error)
	SetWorkerSleepSchedule(ctx context.Context, workerUUID string, schedule *persistence.SleepSchedule) error
	// FetchSleepScheduleWorker sets the given schedule's `Worker` pointer.
	FetchSleepScheduleWorker(ctx context.Context, schedule *persistence.SleepSchedule) error
	FetchSleepSchedulesToCheck(ctx context.Context) ([]*persistence.SleepSchedule, error)

	SetWorkerSleepScheduleNextCheck(ctx context.Context, schedule *persistence.SleepSchedule) error

	SaveWorkerStatus(ctx context.Context, w *persistence.Worker) error
}

var _ PersistenceService = (*persistence.DB)(nil)

// TODO: Refactor the way worker status changes are handled, so that this
// service doens't need to broadcast its own worker updates.
type ChangeBroadcaster interface {
	BroadcastWorkerUpdate(workerUpdate api.SocketIOWorkerUpdate)
}

// ChangeBroadcaster should be a subset of eventbus.Broker.
var _ ChangeBroadcaster = (*eventbus.Broker)(nil)
