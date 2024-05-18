// Package persistence provides the database interface for Flamenco Manager.
package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projects.blender.org/studio/flamenco/internal/uuid"
	"projects.blender.org/studio/flamenco/pkg/api"
)

func TestCreateFetchWorker(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(t, 1*time.Second)
	defer cancel()

	// Test fetching non-existent worker
	fetchedWorker, err := db.FetchWorker(ctx, "dabf67a1-b591-4232-bf73-0b8de2a9488e")
	assert.ErrorIs(t, err, ErrWorkerNotFound)
	assert.Nil(t, fetchedWorker)

	// Test existing worker
	w := Worker{
		UUID:               uuid.New(),
		Name:               "дрон",
		Address:            "fe80::5054:ff:fede:2ad7",
		Platform:           "linux",
		Software:           "3.0",
		Status:             api.WorkerStatusAwake,
		SupportedTaskTypes: "blender,ffmpeg,file-management",
	}

	err = db.CreateWorker(ctx, &w)
	require.NoError(t, err)

	fetchedWorker, err = db.FetchWorker(ctx, w.UUID)
	require.NoError(t, err)
	assert.NotNil(t, fetchedWorker)

	// Test contents of fetched job
	assert.Equal(t, w.UUID, fetchedWorker.UUID)
	assert.Equal(t, w.Name, fetchedWorker.Name)
	assert.Equal(t, w.Address, fetchedWorker.Address)
	assert.Equal(t, w.Platform, fetchedWorker.Platform)
	assert.Equal(t, w.Software, fetchedWorker.Software)
	assert.Equal(t, w.Status, fetchedWorker.Status)

	assert.EqualValues(t, w.SupportedTaskTypes, fetchedWorker.SupportedTaskTypes)
}

func TestFetchWorkerTask(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(t, 10*time.Second)
	defer cancel()

	// Worker without task.
	w := Worker{
		UUID:               uuid.New(),
		Name:               "дрон",
		Address:            "fe80::5054:ff:fede:2ad7",
		Platform:           "linux",
		Software:           "3.0",
		Status:             api.WorkerStatusAwake,
		SupportedTaskTypes: "blender,ffmpeg,file-management",
	}

	err := db.CreateWorker(ctx, &w)
	require.NoError(t, err)

	{ // Test without any task assigned.
		task, err := db.FetchWorkerTask(ctx, &w)
		require.NoError(t, err)
		assert.Nil(t, task)
	}

	// Create a job with tasks.
	authTask1 := authorTestTask("the task", "blender")
	authTask2 := authorTestTask("the other task", "blender")
	jobUUID := "b6a1d859-122f-4791-8b78-b943329a9989"
	atj := authorTestJob(jobUUID, "simple-blender-render", authTask1, authTask2)
	constructTestJob(ctx, t, db, atj)

	assignedTask, err := db.ScheduleTask(ctx, &w)
	require.NoError(t, err)

	{ // Assigned task should be returned.
		foundTask, err := db.FetchWorkerTask(ctx, &w)
		require.NoError(t, err)
		require.NotNil(t, foundTask)
		assert.Equal(t, assignedTask.UUID, foundTask.UUID)
		assert.Equal(t, jobUUID, foundTask.Job.UUID, "the job UUID should be returned as well")
	}

	// Set the task to 'completed'.
	assignedTask.Status = api.TaskStatusCompleted
	require.NoError(t, db.SaveTaskStatus(ctx, assignedTask))

	{ // Completed-but-last-assigned task should be returned.
		foundTask, err := db.FetchWorkerTask(ctx, &w)
		require.NoError(t, err)
		require.NotNil(t, foundTask)
		assert.Equal(t, assignedTask.UUID, foundTask.UUID)
		assert.Equal(t, jobUUID, foundTask.Job.UUID, "the job UUID should be returned as well")
	}

	// Assign another task.
	newlyAssignedTask, err := db.ScheduleTask(ctx, &w)
	require.NoError(t, err)
	require.NotNil(t, newlyAssignedTask)

	{ // Newly assigned task should be returned.
		foundTask, err := db.FetchWorkerTask(ctx, &w)
		require.NoError(t, err)
		require.NotNil(t, foundTask)
		assert.Equal(t, newlyAssignedTask.UUID, foundTask.UUID)
		assert.Equal(t, jobUUID, foundTask.Job.UUID, "the job UUID should be returned as well")
	}

	// Set the new task to 'completed'.
	newlyAssignedTask.Status = api.TaskStatusCompleted
	require.NoError(t, db.SaveTaskStatus(ctx, newlyAssignedTask))

	{ // Completed-but-last-assigned task should be returned.
		foundTask, err := db.FetchWorkerTask(ctx, &w)
		require.NoError(t, err)
		require.NotNil(t, foundTask)
		assert.Equal(t, newlyAssignedTask.UUID, foundTask.UUID)
		assert.Equal(t, jobUUID, foundTask.Job.UUID, "the job UUID should be returned as well")
	}

}

func TestSaveWorker(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(t, 1*time.Second)
	defer cancel()

	w := Worker{
		UUID:               uuid.New(),
		Name:               "дрон",
		Address:            "fe80::5054:ff:fede:2ad7",
		Platform:           "linux",
		Software:           "3.0",
		Status:             api.WorkerStatusAwake,
		SupportedTaskTypes: "blender,ffmpeg,file-management",
	}

	err := db.CreateWorker(ctx, &w)
	require.NoError(t, err)

	fetchedWorker, err := db.FetchWorker(ctx, w.UUID)
	require.NoError(t, err)
	assert.NotNil(t, fetchedWorker)

	// Update all updatable fields of the Worker
	updatedWorker := *fetchedWorker
	updatedWorker.Name = "7 မှ 9"
	updatedWorker.Address = "fe80::cafe:f00d"
	updatedWorker.Platform = "windows"
	updatedWorker.Software = "3.1"
	updatedWorker.Status = api.WorkerStatusAsleep
	updatedWorker.SupportedTaskTypes = "blender,ffmpeg,file-management,misc"

	// Saving only the status should just do that.
	err = db.SaveWorkerStatus(ctx, &updatedWorker)
	require.NoError(t, err)
	assert.Equal(t, "7 မှ 9", updatedWorker.Name, "Saving status should not touch the name")

	// Check saved worker
	fetchedWorker, err = db.FetchWorker(ctx, w.UUID)
	require.NoError(t, err)
	assert.NotNil(t, fetchedWorker)
	assert.Equal(t, updatedWorker.Status, fetchedWorker.Status, "new status should have been saved")
	assert.NotEqual(t, updatedWorker.Name, fetchedWorker.Name, "non-status fields should not have been updated")

	// Saving the entire worker should save everything.
	err = db.SaveWorker(ctx, &updatedWorker)
	require.NoError(t, err)

	// Check saved worker
	fetchedWorker, err = db.FetchWorker(ctx, w.UUID)
	require.NoError(t, err)
	assert.NotNil(t, fetchedWorker)
	assert.Equal(t, updatedWorker.Status, fetchedWorker.Status, "new status should have been saved")
	assert.Equal(t, updatedWorker.Name, fetchedWorker.Name, "non-status fields should also have been updated")
	assert.Equal(t, updatedWorker.Software, fetchedWorker.Software, "non-status fields should also have been updated")
}

func TestFetchWorkers(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(t, 1*time.Second)
	defer cancel()

	// No workers
	workers, err := db.FetchWorkers(ctx)
	require.NoError(t, err)
	require.Empty(t, workers)

	linuxWorker := Worker{
		UUID:               uuid.New(),
		Name:               "дрон",
		Address:            "fe80::5054:ff:fede:2ad7",
		Platform:           "linux",
		Software:           "3.0",
		Status:             api.WorkerStatusAwake,
		SupportedTaskTypes: "blender,ffmpeg,file-management",
	}

	// One worker:
	err = db.CreateWorker(ctx, &linuxWorker)
	require.NoError(t, err)
	assert.Equal(t, time.Now().UTC().Location(), linuxWorker.CreatedAt.Location(),
		"Timestamps should be using UTC timezone")

	workers, err = db.FetchWorkers(ctx)
	require.NoError(t, err)
	if assert.Len(t, workers, 1) {
		// FIXME: this fails, because the fetched timestamps have nil location instead of UTC.
		// assert.Equal(t, time.Now().UTC().Location(), workers[0].CreatedAt.Location(),
		// 	"Timestamps should be using UTC timezone")

		assert.Equal(t, linuxWorker.UUID, workers[0].UUID)
		assert.Equal(t, linuxWorker.Name, workers[0].Name)
		assert.Equal(t, linuxWorker.Address, workers[0].Address)
		assert.Equal(t, linuxWorker.Status, workers[0].Status)
		assert.Equal(t, linuxWorker.SupportedTaskTypes, workers[0].SupportedTaskTypes)
	}

	// Two workers:
	windowsWorker := Worker{
		UUID:               uuid.New(),
		Name:               "очиститель окон",
		Address:            "fe80::c000:d000:::3",
		Platform:           "windows",
		Software:           "3.0",
		Status:             api.WorkerStatusOffline,
		SupportedTaskTypes: "blender,ffmpeg,file-management",
	}
	err = db.CreateWorker(ctx, &windowsWorker)
	require.NoError(t, err)

	workers, err = db.FetchWorkers(ctx)
	require.NoError(t, err)
	if assert.Len(t, workers, 2) {
		assert.Equal(t, linuxWorker.UUID, workers[0].UUID)
		assert.Equal(t, windowsWorker.UUID, workers[1].UUID)
	}
}

func TestDeleteWorker(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(t, 1*time.Second)
	defer cancel()

	// Test deleting non-existent worker
	err := db.DeleteWorker(ctx, "dabf67a1-b591-4232-bf73-0b8de2a9488e")
	assert.ErrorIs(t, err, ErrWorkerNotFound)

	// Test deleting existing worker
	w1 := Worker{
		UUID:   "fd97a35b-a5bd-44b4-ac2b-64c193ca877d",
		Name:   "Worker 1",
		Status: api.WorkerStatusAwake,
	}
	w2 := Worker{
		UUID:   "82b2d176-cb8c-4bfa-8300-41c216d766df",
		Name:   "Worker  2",
		Status: api.WorkerStatusOffline,
	}

	require.NoError(t, db.CreateWorker(ctx, &w1))
	require.NoError(t, db.CreateWorker(ctx, &w2))

	// Delete the 2nd worker, just to have a test with ID != 1.
	require.NoError(t, db.DeleteWorker(ctx, w2.UUID))

	// The deleted worker should now no longer be found.
	{
		fetchedWorker, err := db.FetchWorker(ctx, w2.UUID)
		assert.ErrorIs(t, err, ErrWorkerNotFound)
		assert.Nil(t, fetchedWorker)
	}

	// The other worker should still exist.
	{
		fetchedWorker, err := db.FetchWorker(ctx, w1.UUID)
		require.NoError(t, err)
		assert.Equal(t, w1.UUID, fetchedWorker.UUID)
	}

	// Assign a task to the other worker, and then delete that worker.
	authJob := createTestAuthoredJobWithTasks()
	persistAuthoredJob(t, ctx, db, authJob)
	taskUUID := authJob.Tasks[0].UUID
	{
		task, err := db.FetchTask(ctx, taskUUID)
		require.NoError(t, err)
		task.Worker = &w1
		require.NoError(t, db.SaveTask(ctx, task))
	}

	// Delete the worker.
	require.NoError(t, db.DeleteWorker(ctx, w1.UUID))

	// Check the task after deletion of the Worker.
	{
		fetchedTask, err := db.FetchTask(ctx, taskUUID)
		require.NoError(t, err)
		assert.Equal(t, taskUUID, fetchedTask.UUID)
		assert.Equal(t, w1.UUID, fetchedTask.Worker.UUID)
		assert.NotZero(t, fetchedTask.Worker.DeletedAt.Time)
		assert.True(t, fetchedTask.Worker.DeletedAt.Valid)
	}
}

func TestDeleteWorkerNoForeignKeys(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(t, 1*time.Second)
	defer cancel()

	// Create a Worker to delete.
	w1 := Worker{
		UUID:   "fd97a35b-a5bd-44b4-ac2b-64c193ca877d",
		Name:   "Worker 1",
		Status: api.WorkerStatusAwake,
	}
	require.NoError(t, db.CreateWorker(ctx, &w1))

	// Try deleting with foreign key constraints disabled.
	require.NoError(t, db.pragmaForeignKeys(false))
	require.ErrorIs(t, ErrDeletingWithoutFK, db.DeleteWorker(ctx, w1.UUID))

	// The worker should still exist.
	{
		fetchedWorker, err := db.FetchWorker(ctx, w1.UUID)
		require.NoError(t, err)
		assert.Equal(t, w1.UUID, fetchedWorker.UUID)
	}
}

func TestDeleteWorkerWithTagAssigned(t *testing.T) {
	f := workerTestFixtures(t, 1*time.Second)
	defer f.done()

	// Assign the worker.
	require.NoError(t, f.db.WorkerSetTags(f.ctx, f.worker, []string{f.tag.UUID}))

	// Delete the Worker.
	require.NoError(t, f.db.DeleteWorker(f.ctx, f.worker.UUID))

	// Check the Worker has been unassigned from the tag.
	tag, err := f.db.FetchWorkerTag(f.ctx, f.tag.UUID)
	require.NoError(t, err)
	assert.Empty(t, tag.Workers)
}

func TestSummarizeWorkerStatuses(t *testing.T) {
	f := workerTestFixtures(t, 1*time.Second)
	defer f.done()

	// Test the summary.
	summary, err := f.db.SummarizeWorkerStatuses(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, WorkerStatusCount{api.WorkerStatusAwake: 1}, summary)

	// Create more workers.
	w1 := Worker{
		UUID:   "fd97a35b-a5bd-44b4-ac2b-64c193ca877d",
		Name:   "Worker 1",
		Status: api.WorkerStatusAwake,
	}
	w2 := Worker{
		UUID:   "82b2d176-cb8c-4bfa-8300-41c216d766df",
		Name:   "Worker  2",
		Status: api.WorkerStatusOffline,
	}

	require.NoError(t, f.db.CreateWorker(f.ctx, &w1))
	require.NoError(t, f.db.CreateWorker(f.ctx, &w2))

	// Test the summary.
	summary, err = f.db.SummarizeWorkerStatuses(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, WorkerStatusCount{
		api.WorkerStatusAwake:   2,
		api.WorkerStatusOffline: 1,
	}, summary)

	// Delete all workers.
	require.NoError(t, f.db.DeleteWorker(f.ctx, f.worker.UUID))
	require.NoError(t, f.db.DeleteWorker(f.ctx, w1.UUID))
	require.NoError(t, f.db.DeleteWorker(f.ctx, w2.UUID))

	// Test the summary.
	summary, err = f.db.SummarizeWorkerStatuses(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, WorkerStatusCount{}, summary)
}

// Check that a context timeout can be detected by inspecting the
// returned error.
func TestSummarizeWorkerStatusesTimeout(t *testing.T) {
	f := workerTestFixtures(t, 1*time.Second)
	defer f.done()

	subCtx, subCtxCancel := context.WithTimeout(f.ctx, 1*time.Nanosecond)
	defer subCtxCancel()

	// Force a timeout of the context. And yes, even when a nanosecond is quite
	// short, it is still necessary to wait.
	time.Sleep(1 * time.Millisecond)

	// Test the summary.
	summary, err := f.db.SummarizeWorkerStatuses(subCtx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, summary)
}
