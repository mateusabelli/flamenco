package sleep_scheduler

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/internal/manager/sleep_scheduler/mocks"
	"projects.blender.org/studio/flamenco/pkg/api"
	"projects.blender.org/studio/flamenco/pkg/time_of_day"
)

func TestFetchSchedule(t *testing.T) {
	ss, mocks, ctx := testFixtures(t)

	workerUUID := "aeb49d8a-6903-41b3-b545-77b7a1c0ca19"
	dbSched := persistence.SleepSchedule{}
	mocks.persist.EXPECT().FetchWorkerSleepSchedule(ctx, workerUUID).Return(&dbSched, nil)

	sched, err := ss.FetchSchedule(ctx, workerUUID)
	require.NoError(t, err)
	assert.Equal(t, &dbSched, sched)
}

func TestSetSchedule(t *testing.T) {
	ss, mocks, ctx := testFixtures(t)

	workerUUID := "aeb49d8a-6903-41b3-b545-77b7a1c0ca19"
	worker := persistence.Worker{
		UUID:   workerUUID,
		Status: api.WorkerStatusAwake,
	}

	sched := persistence.SleepSchedule{
		IsActive:   true,
		DaysOfWeek: " mo  tu  we",
		StartTime:  time_of_day.New(9, 0),
		EndTime:    time_of_day.New(18, 0),
		WorkerID:   worker.ID,
	}
	expectSavedSchedule := sched
	expectSavedSchedule.DaysOfWeek = "mo tu we" // Expect a cleanup
	expectNextCheck := mocks.todayAt(18, 0)     // "now" is at 11:14:47, expect a check at the end time.
	expectSavedSchedule.SetNextCheck(expectNextCheck)

	mocks.persist.EXPECT().FetchSleepScheduleWorker(ctx, expectSavedSchedule).Return(&worker, nil)

	// Expect the new schedule to be saved.
	mocks.persist.EXPECT().SetWorkerSleepSchedule(ctx, workerUUID, &expectSavedSchedule)

	// Expect the new schedule to be immediately applied to the Worker.
	// `TestApplySleepSchedule` checks those values, no need to do that here.
	mocks.persist.EXPECT().SaveWorkerStatus(ctx, gomock.Any())
	mocks.broadcaster.EXPECT().BroadcastWorkerUpdate(gomock.Any())

	err := ss.SetSchedule(ctx, workerUUID, sched)
	require.NoError(t, err)
}

func TestSetScheduleSwappedStartEnd(t *testing.T) {
	ss, mocks, ctx := testFixtures(t)

	// Worker already in the right state, so no saving/broadcasting expected.
	worker := persistence.Worker{
		ID:     47,
		UUID:   "aeb49d8a-6903-41b3-b545-77b7a1c0ca19",
		Name:   "test worker",
		Status: api.WorkerStatusAsleep,
	}
	sched := persistence.SleepSchedule{
		IsActive:   true,
		DaysOfWeek: "mo tu we",
		StartTime:  time_of_day.New(18, 0),
		EndTime:    time_of_day.New(9, 0),
		WorkerID:   worker.ID,
	}

	expectSavedSchedule := persistence.SleepSchedule{
		IsActive:   true,
		DaysOfWeek: "mo tu we",
		StartTime:  time_of_day.New(9, 0), // Expect start and end time to be corrected.
		EndTime:    time_of_day.New(18, 0),
		NextCheck:  sql.NullTime{Time: mocks.todayAt(18, 0), Valid: true}, // "now" is at 11:14:47, expect a check at the end time.
		WorkerID:   worker.ID,
	}

	mocks.persist.EXPECT().FetchSleepScheduleWorker(ctx, expectSavedSchedule).Return(&worker, nil)
	mocks.persist.EXPECT().SetWorkerSleepSchedule(ctx, worker.UUID, &expectSavedSchedule)

	err := ss.SetSchedule(ctx, worker.UUID, sched)
	require.NoError(t, err)
}

// Test that a sleep check that happens at shutdown of the Manager doesn't cause any panics.
func TestCheckSleepScheduleAtShutdown(t *testing.T) {
	ss, mocks, _ := testFixtures(t)

	worker := persistence.Worker{
		ID:     47,
		UUID:   "aeb49d8a-6903-41b3-b545-77b7a1c0ca19",
		Name:   "test worker",
		Status: api.WorkerStatusAsleep,
	}

	sched := persistence.SleepSchedule{
		IsActive:   true,
		DaysOfWeek: "mo tu we",
		StartTime:  time_of_day.New(18, 0),
		EndTime:    time_of_day.New(9, 0),
	}

	// Construct the updated-and-about-to-be-saved schedule.
	updatedSched := sched
	updatedSched.NextCheck = sql.NullTime{
		Time:  sched.StartTime.OnDate(mocks.clock.Now()),
		Valid: true,
	}

	// Cancel the context to mimick the Manager shutting down.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mocks.persist.EXPECT().SetWorkerSleepScheduleNextCheck(ctx, updatedSched).Return(context.Canceled)
	ss.checkSchedule(ctx, persistence.SleepScheduleOwned{
		SleepSchedule: sched,
		WorkerName:    worker.Name,
		WorkerUUID:    worker.UUID,
	})
}

func TestApplySleepSchedule(t *testing.T) {
	ss, mocks, ctx := testFixtures(t)

	worker := persistence.Worker{
		ID:     5,
		UUID:   "74997de4-c530-4913-b89f-c489f14f7634",
		Status: api.WorkerStatusOffline,
	}

	sched := persistence.SleepSchedule{
		IsActive:   true,
		DaysOfWeek: "mo tu we",
		StartTime:  time_of_day.New(9, 0),
		EndTime:    time_of_day.New(18, 0),
	}

	testForExpectedStatus := func(expectedNewStatus api.WorkerStatus) {
		// Take a copy of the worker & schedule, for test isolation.
		testSchedule := sched
		testWorker := worker

		// Expect the Worker to be fetched.
		mocks.persist.EXPECT().FetchSleepScheduleWorker(ctx, testSchedule).Return(&testWorker, nil)

		// Construct the worker as we expect it to be saved to the database.
		savedWorker := testWorker
		savedWorker.LazyStatusRequest = false
		savedWorker.StatusRequested = expectedNewStatus
		mocks.persist.EXPECT().SaveWorkerStatus(ctx, &savedWorker)

		// Expect SocketIO broadcast.
		var sioUpdate api.EventWorkerUpdate
		mocks.broadcaster.EXPECT().BroadcastWorkerUpdate(gomock.Any()).DoAndReturn(
			func(workerUpdate api.EventWorkerUpdate) {
				sioUpdate = workerUpdate
			})

		// Actually apply the sleep schedule.
		err := ss.ApplySleepSchedule(ctx, testSchedule)
		require.NoError(t, err)

		// Check the SocketIO broadcast.
		if sioUpdate.Id != "" {
			assert.Equal(t, testWorker.UUID, sioUpdate.Id)
			assert.False(t, sioUpdate.StatusChange.IsLazy)
			assert.Equal(t, expectedNewStatus, sioUpdate.StatusChange.Status)
		}
	}

	// Move the clock to the middle of the sleep schedule, so worker should sleep.
	mocks.clock.Set(mocks.todayAt(10, 47))
	testForExpectedStatus(api.WorkerStatusAsleep)

	// Move the clock to before the sleep schedule start.
	mocks.clock.Set(mocks.todayAt(0, 3))
	testForExpectedStatus(api.WorkerStatusAwake)

	// Move the clock to after the sleep schedule ends.
	mocks.clock.Set(mocks.todayAt(19, 59))
	testForExpectedStatus(api.WorkerStatusAwake)

	// Test that the worker should sleep, and has already been requested to sleep,
	// but lazily. This should trigger a non-lazy status change request.
	mocks.clock.Set(mocks.todayAt(10, 47))
	worker.Status = api.WorkerStatusAwake
	worker.StatusRequested = api.WorkerStatusAsleep
	worker.LazyStatusRequest = true
	testForExpectedStatus(api.WorkerStatusAsleep)
}

func TestApplySleepScheduleNoStatusChange(t *testing.T) {
	ss, mocks, ctx := testFixtures(t)

	worker := persistence.Worker{
		ID:     5,
		UUID:   "74997de4-c530-4913-b89f-c489f14f7634",
		Status: api.WorkerStatusAsleep,
	}

	sched := persistence.SleepSchedule{
		IsActive:   true,
		DaysOfWeek: "mo tu we",
		StartTime:  time_of_day.New(9, 0),
		EndTime:    time_of_day.New(18, 0),
	}

	runTest := func() {
		// Take a copy of the worker & schedule, for test isolation.
		testSchedule := sched
		testWorker := worker

		// Expect the Worker to be fetched.
		mocks.persist.EXPECT().FetchSleepScheduleWorker(ctx, testSchedule).Return(&testWorker, nil)

		// Apply the sleep schedule. This should not trigger any persistence or broadcasts.
		err := ss.ApplySleepSchedule(ctx, testSchedule)
		require.NoError(t, err)
	}

	// Move the clock to the middle of the sleep schedule, so the schedule always
	// wants the worker to sleep.
	mocks.clock.Set(mocks.todayAt(10, 47))

	// Current status is already good.
	worker.Status = api.WorkerStatusAsleep
	runTest()

	// Current status is not the right one, but the requested status is already good.
	worker.Status = api.WorkerStatusAwake
	worker.StatusRequested = api.WorkerStatusAsleep
	worker.LazyStatusRequest = false
	runTest()

	// Current status is not the right one, but error state should not be overwrittne.
	worker.Status = api.WorkerStatusError
	worker.StatusRequested = ""
	worker.LazyStatusRequest = false
	runTest()
}

type TestMocks struct {
	clock       *clock.Mock
	persist     *mocks.MockPersistenceService
	broadcaster *mocks.MockChangeBroadcaster
}

// todayAt returns whatever the mocked clock's "now" is set to, with the time set
// to the given time. Seconds and sub-seconds are set to zero.
func (m *TestMocks) todayAt(hour, minute int) time.Time {
	now := m.clock.Now()
	todayAt := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.Local)
	return todayAt
}

// endOfDay returns midnight of the day after whatever the mocked clock's "now" is set to.
func (m *TestMocks) endOfDay() time.Time {
	startOfToday := m.todayAt(0, 0)
	return startOfToday.AddDate(0, 0, 1)
}

func testFixtures(t *testing.T) (*SleepScheduler, TestMocks, context.Context) {
	ctx := context.Background()

	mockedClock := clock.NewMock()
	mockedNow, err := time.Parse(time.RFC3339, "2022-06-07T11:14:47+02:00")
	require.NoError(t, err)

	mockedClock.Set(mockedNow)
	if !assert.Equal(t, time.Tuesday.String(), mockedNow.Weekday().String()) {
		t.Fatal("tests assume 'now' is a Tuesday")
	}

	mockCtrl := gomock.NewController(t)
	mocks := TestMocks{
		clock:       mockedClock,
		persist:     mocks.NewMockPersistenceService(mockCtrl),
		broadcaster: mocks.NewMockChangeBroadcaster(mockCtrl),
	}
	ss := New(mocks.clock, mocks.persist, mocks.broadcaster)
	return ss, mocks, ctx
}
