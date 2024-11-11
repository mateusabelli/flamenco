package timeout_checker

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"database/sql"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

const workerTimeout = 20 * time.Minute

func TestWorkerTimeout(t *testing.T) {
	canaryTest(t)

	ttc, finish, mocks := timeoutCheckerTestFixtures(t)
	defer finish()

	mocks.run(ttc)

	// Wait for the timeout checker to actually be sleeping, otherwise it could
	// have a different sleep-start time than we expect.
	time.Sleep(1 * time.Millisecond)

	lastSeenAt := mocks.clock.Now().UTC().Add(-1 * time.Hour)

	worker := persistence.Worker{
		UUID:            "WORKER-UUID",
		Name:            "Tester",
		ID:              47,
		LastSeenAt:      sql.NullTime{Time: lastSeenAt, Valid: true},
		Status:          api.WorkerStatusAsleep,
		StatusRequested: api.WorkerStatusAwake,
	}

	// No tasks are timing out in this test.
	mocks.persist.EXPECT().FetchTimedOutTasks(mocks.ctx, gomock.Any()).Return([]*persistence.Task{}, nil)

	mocks.persist.EXPECT().FetchTimedOutWorkers(mocks.ctx, gomock.Any()).
		Return([]*persistence.Worker{&worker}, nil)

	// Expect all tasks assigned to the worker to get requeued.
	mocks.taskStateMachine.EXPECT().RequeueActiveTasksOfWorker(mocks.ctx, &worker, "worker timed out")

	persistedWorker := worker
	persistedWorker.Status = api.WorkerStatusError
	// Any queued up status change should be cleared, as the Worker is not allowed
	// to change into anything until this timeout has been address.
	persistedWorker.StatusChangeClear()
	mocks.persist.EXPECT().SaveWorker(mocks.ctx, &persistedWorker).Return(nil)

	prevStatus := worker.Status
	mocks.broadcaster.EXPECT().BroadcastWorkerUpdate(api.EventWorkerUpdate{
		Id:             worker.UUID,
		Name:           worker.Name,
		PreviousStatus: &prevStatus,
		Status:         api.WorkerStatusError,
		Updated:        persistedWorker.UpdatedAt.Time,
		Version:        persistedWorker.Software,
	})

	// All the timeouts should be handled after the initial sleep.
	mocks.clock.Add(timeoutInitialSleep)
}
