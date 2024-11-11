package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"projects.blender.org/studio/flamenco/internal/uuid"
	"projects.blender.org/studio/flamenco/pkg/api"
	"projects.blender.org/studio/flamenco/pkg/time_of_day"
)

func TestFetchWorkerSleepSchedule(t *testing.T) {
	ctx, finish, db := persistenceTestFixtures(1 * time.Second)
	defer finish()

	linuxWorker := Worker{
		UUID:               uuid.New(),
		Name:               "дрон",
		Address:            "fe80::5054:ff:fede:2ad7",
		Platform:           "linux",
		Software:           "3.0",
		Status:             api.WorkerStatusAwake,
		SupportedTaskTypes: "blender,ffmpeg,file-management",
	}
	err := db.CreateWorker(ctx, &linuxWorker)
	require.NoError(t, err)

	// Not an existing Worker.
	fetched, err := db.FetchWorkerSleepSchedule(ctx, "2cf6153a-3d4e-49f4-a5c0-1c9fc176e155")
	require.NoError(t, err, "non-existent worker should not cause an error")
	assert.Zero(t, fetched)

	// No sleep schedule.
	fetched, err = db.FetchWorkerSleepSchedule(ctx, linuxWorker.UUID)
	require.NoError(t, err, "non-existent schedule should not cause an error")
	assert.Zero(t, fetched)

	// Create a sleep schedule.
	created := SleepSchedule{
		WorkerID:   linuxWorker.ID,
		IsActive:   true,
		DaysOfWeek: "mo,tu,th,fr",
		StartTime:  time_of_day.New(18, 0),
		EndTime:    time_of_day.New(9, 0),
	}
	err = db.SetWorkerSleepSchedule(ctx, linuxWorker.UUID, &created)
	require.NoError(t, err)

	fetched, err = db.FetchWorkerSleepSchedule(ctx, linuxWorker.UUID)
	require.NoError(t, err)
	assertEqualSleepSchedule(t, linuxWorker.ID, created, *fetched)
}

func TestFetchSleepScheduleWorker(t *testing.T) {
	ctx, finish, db := persistenceTestFixtures(1 * time.Second)
	defer finish()

	linuxWorker := Worker{
		UUID:               uuid.New(),
		Name:               "дрон",
		Address:            "fe80::5054:ff:fede:2ad7",
		Platform:           "linux",
		Software:           "3.0",
		Status:             api.WorkerStatusAwake,
		SupportedTaskTypes: "blender,ffmpeg,file-management",
	}
	err := db.CreateWorker(ctx, &linuxWorker)
	require.NoError(t, err)

	// Create a sleep schedule.
	created := SleepSchedule{
		WorkerID:   linuxWorker.ID,
		IsActive:   true,
		DaysOfWeek: "mo,tu,th,fr",
		StartTime:  time_of_day.New(18, 0),
		EndTime:    time_of_day.New(9, 0),
	}
	err = db.SetWorkerSleepSchedule(ctx, linuxWorker.UUID, &created)
	require.NoError(t, err)

	dbSchedule, err := db.FetchWorkerSleepSchedule(ctx, linuxWorker.UUID)
	require.NoError(t, err)
	require.NotNil(t, dbSchedule)

	worker, err := db.FetchSleepScheduleWorker(ctx, *dbSchedule)
	require.NoError(t, err)
	if assert.NotNil(t, worker) {
		// Compare a few fields. If these are good, the correct worker has been fetched.
		assert.Equal(t, linuxWorker.ID, worker.ID)
		assert.Equal(t, linuxWorker.UUID, worker.UUID)
	}

	// Deleting the Worker should result in a specific error when fetching the schedule again.
	require.NoError(t, db.DeleteWorker(ctx, linuxWorker.UUID))
	worker, err = db.FetchSleepScheduleWorker(ctx, *dbSchedule)
	assert.ErrorIs(t, err, ErrWorkerNotFound)
	assert.Nil(t, worker)
}

func TestSetWorkerSleepSchedule(t *testing.T) {
	ctx, finish, db := persistenceTestFixtures(1 * time.Second)
	defer finish()

	linuxWorker := Worker{
		UUID:               uuid.New(),
		Name:               "дрон",
		Address:            "fe80::5054:ff:fede:2ad7",
		Platform:           "linux",
		Software:           "3.0",
		Status:             api.WorkerStatusAwake,
		SupportedTaskTypes: "blender,ffmpeg,file-management",
	}
	err := db.CreateWorker(ctx, &linuxWorker)
	require.NoError(t, err)

	schedule := SleepSchedule{
		WorkerID:   linuxWorker.ID,
		IsActive:   true,
		DaysOfWeek: "mo,tu,th,fr",
		StartTime:  time_of_day.New(18, 0),
		EndTime:    time_of_day.New(9, 0),
	}

	// Not an existing Worker.
	err = db.SetWorkerSleepSchedule(ctx, "2cf6153a-3d4e-49f4-a5c0-1c9fc176e155", &schedule)
	assert.ErrorIs(t, err, ErrWorkerNotFound)

	// Create the sleep schedule.
	err = db.SetWorkerSleepSchedule(ctx, linuxWorker.UUID, &schedule)
	require.NoError(t, err)
	fetched, err := db.FetchWorkerSleepSchedule(ctx, linuxWorker.UUID)
	require.NoError(t, err)
	assertEqualSleepSchedule(t, linuxWorker.ID, schedule, *fetched)

	// Overwrite the schedule with one that already has a database ID.
	newSchedule := schedule
	newSchedule.IsActive = false
	newSchedule.DaysOfWeek = "mo,tu,we,th,fr"
	newSchedule.StartTime = time_of_day.New(2, 0)
	newSchedule.EndTime = time_of_day.New(6, 0)
	err = db.SetWorkerSleepSchedule(ctx, linuxWorker.UUID, &newSchedule)
	require.NoError(t, err)
	fetched, err = db.FetchWorkerSleepSchedule(ctx, linuxWorker.UUID)
	require.NoError(t, err)
	assertEqualSleepSchedule(t, linuxWorker.ID, newSchedule, *fetched)

	// Overwrite the schedule with a freshly constructed one.
	newerSchedule := SleepSchedule{
		WorkerID:   linuxWorker.ID,
		IsActive:   true,
		DaysOfWeek: "mo",
		StartTime:  time_of_day.New(3, 0),
		EndTime:    time_of_day.New(15, 0),
	}
	err = db.SetWorkerSleepSchedule(ctx, linuxWorker.UUID, &newerSchedule)
	require.NoError(t, err)
	fetched, err = db.FetchWorkerSleepSchedule(ctx, linuxWorker.UUID)
	require.NoError(t, err)
	assertEqualSleepSchedule(t, linuxWorker.ID, newerSchedule, *fetched)

	// Clear the sleep schedule.
	emptySchedule := SleepSchedule{
		WorkerID:   linuxWorker.ID,
		IsActive:   false,
		DaysOfWeek: "",
		StartTime:  time_of_day.Empty(),
		EndTime:    time_of_day.Empty(),
	}
	err = db.SetWorkerSleepSchedule(ctx, linuxWorker.UUID, &emptySchedule)
	require.NoError(t, err)
	fetched, err = db.FetchWorkerSleepSchedule(ctx, linuxWorker.UUID)
	require.NoError(t, err)
	assertEqualSleepSchedule(t, linuxWorker.ID, emptySchedule, *fetched)
}

func TestSetWorkerSleepScheduleNextCheck(t *testing.T) {
	ctx, finish, db := persistenceTestFixtures(1 * time.Second)
	defer finish()

	w := linuxWorker(t, db, func(worker *Worker) {
		worker.Status = api.WorkerStatusAwake
	})

	schedule := SleepSchedule{
		IsActive:   true,
		DaysOfWeek: "mo,tu,th,fr",
		StartTime:  time_of_day.New(18, 0),
		EndTime:    time_of_day.New(9, 0),
	}
	err := db.SetWorkerSleepSchedule(ctx, w.UUID, &schedule)
	require.NoError(t, err)

	future := db.now().Add(5 * time.Hour)
	schedule.NextCheck = sql.NullTime{Time: future, Valid: true}

	err = db.SetWorkerSleepScheduleNextCheck(ctx, schedule)
	require.NoError(t, err)

	fetched, err := db.FetchWorkerSleepSchedule(ctx, w.UUID)
	require.NoError(t, err)
	assertEqualSleepSchedule(t, w.ID, schedule, *fetched)
}

func TestFetchSleepSchedulesToCheck(t *testing.T) {
	ctx, finish, db := persistenceTestFixtures(1 * time.Second)
	defer finish()

	mockedNow := mustParseTime("2022-06-07T11:14:47+02:00").UTC()
	mockedPast := mockedNow.Add(-10 * time.Second)
	mockedFuture := mockedNow.Add(10 * time.Second)

	db.nowfunc = func() time.Time { return mockedNow }

	worker0 := createWorker(ctx, t, db, func(w *Worker) {
		w.UUID = "2b1f857a-fd64-484b-9c17-cf89bbe47be7"
		w.Name = "дрон 1"
		w.Status = api.WorkerStatusAwake
	})
	worker1 := createWorker(ctx, t, db, func(w *Worker) {
		w.UUID = "4475738e-41eb-47b2-8bca-2bbcabab69bb"
		w.Name = "дрон 2"
		w.Status = api.WorkerStatusAwake
	})
	worker2 := createWorker(ctx, t, db, func(w *Worker) {
		w.UUID = "dc251817-6a11-4548-a36a-07b0d50b4c21"
		w.Name = "дрон 3"
		w.Status = api.WorkerStatusAwake
	})
	worker3 := createWorker(ctx, t, db, func(w *Worker) {
		w.UUID = "874d5fc6-5784-4d43-8c20-6e7e73fc1b8d"
		w.Name = "дрон 4"
		w.Status = api.WorkerStatusAwake
	})

	schedule0 := SleepSchedule{ // Next check in the past -> should be checked.
		IsActive:   true,
		DaysOfWeek: "mo,tu,th,fr",
		StartTime:  time_of_day.New(18, 0),
		EndTime:    time_of_day.New(9, 0),
		NextCheck:  sql.NullTime{Time: mockedPast, Valid: true},
	}

	schedule1 := SleepSchedule{ // Next check in future -> should not be checked.
		IsActive:   true,
		DaysOfWeek: "mo,tu,th,fr",
		StartTime:  time_of_day.New(18, 0),
		EndTime:    time_of_day.New(9, 0),
		NextCheck:  sql.NullTime{Time: mockedFuture, Valid: true},
	}

	schedule2 := SleepSchedule{ // Next check is zero value -> should be checked.
		IsActive:   true,
		DaysOfWeek: "mo,tu,th,fr",
		StartTime:  time_of_day.New(18, 0),
		EndTime:    time_of_day.New(9, 0),
		NextCheck:  sql.NullTime{}, // zero value for time.
	}

	schedule3 := SleepSchedule{ // Schedule inactive -> should not be checked.
		IsActive:   false,
		DaysOfWeek: "mo,tu,th,fr",
		StartTime:  time_of_day.New(18, 0),
		EndTime:    time_of_day.New(9, 0),
		NextCheck:  sql.NullTime{Time: mockedPast, Valid: true}, // next check in the past, so if active it would be checked.
	}

	// Create the workers and sleep schedules.
	scheds := []*SleepSchedule{&schedule0, &schedule1, &schedule2, &schedule3}
	workers := []*Worker{worker0, worker1, worker2, worker3}
	for idx := range scheds {
		err := db.SetWorkerSleepSchedule(ctx, workers[idx].UUID, scheds[idx])
		require.NoError(t, err)
	}

	toCheck, err := db.FetchSleepSchedulesToCheck(ctx)
	require.NoError(t, err)
	require.Len(t, toCheck, 2)

	assert.Equal(t, worker0.Name, toCheck[0].WorkerName)
	assert.Equal(t, worker0.UUID, toCheck[0].WorkerUUID)
	assert.Equal(t, worker2.Name, toCheck[1].WorkerName)
	assert.Equal(t, worker2.UUID, toCheck[1].WorkerUUID)
	assertEqualSleepSchedule(t, worker0.ID, schedule0, toCheck[0].SleepSchedule)
	assertEqualSleepSchedule(t, worker2.ID, schedule1, toCheck[1].SleepSchedule)
}

func assertEqualSleepSchedule(t *testing.T, workerID int64, expect, actual SleepSchedule) {
	assert.Equal(t, workerID, actual.WorkerID, "sleep schedule is assigned to different worker")
	assert.Equal(t, expect.IsActive, actual.IsActive, "IsActive does not match")
	assert.Equal(t, expect.DaysOfWeek, actual.DaysOfWeek, "DaysOfWeek does not match")
	assert.Equal(t, expect.StartTime, actual.StartTime, "StartTime does not match")
	assert.Equal(t, expect.EndTime, actual.EndTime, "EndTime does not match")
}

func mustParseTime(timeString string) time.Time {
	parsed, err := time.Parse(time.RFC3339, timeString)
	if err != nil {
		panic(err)
	}
	return parsed
}
