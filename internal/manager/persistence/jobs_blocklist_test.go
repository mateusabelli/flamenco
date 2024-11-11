package persistence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SPDX-License-Identifier: GPL-3.0-or-later

func TestAddWorkerToJobBlocklist(t *testing.T) {
	ctx, close, db, job, _ := jobTasksTestFixtures(t)
	defer close()

	worker := createWorker(ctx, t, db)
	queries := db.queries()

	{
		// Add a worker to the block list.
		err := db.AddWorkerToJobBlocklist(ctx, job, worker, "blender")
		require.NoError(t, err)

		list, err := queries.Test_FetchJobBlocklist(ctx)
		require.NoError(t, err)
		if assert.Len(t, list, 1) {
			entry := list[0]
			assert.Equal(t, int64(job.ID), entry.JobID)
			assert.Equal(t, int64(worker.ID), entry.WorkerID)
			assert.Equal(t, "blender", entry.TaskType)
		}
	}

	{
		// Adding the same worker again should be a no-op.
		err := db.AddWorkerToJobBlocklist(ctx, job, worker, "blender")
		require.NoError(t, err)

		list, err := queries.Test_FetchJobBlocklist(ctx)
		require.NoError(t, err)
		assert.Len(t, list, 1, "No new entry should have been created")
	}
}

func TestFetchJobBlocklist(t *testing.T) {
	ctx, close, db, job, _ := jobTasksTestFixtures(t)
	defer close()

	// Add a worker to the block list.
	worker := createWorker(ctx, t, db)
	err := db.AddWorkerToJobBlocklist(ctx, job, worker, "blender")
	require.NoError(t, err)

	list, err := db.FetchJobBlocklist(ctx, job.UUID)
	require.NoError(t, err)

	if assert.Len(t, list, 1) {
		entry := list[0]
		assert.Equal(t, worker.Name, entry.WorkerName)
		assert.Equal(t, worker.UUID, entry.WorkerUUID)
		assert.Equal(t, "blender", entry.TaskType)
	}
}

func TestClearJobBlocklist(t *testing.T) {
	ctx, close, db, job, _ := jobTasksTestFixtures(t)
	defer close()

	// Add a worker and some entries to the block list.
	worker := createWorker(ctx, t, db)
	err := db.AddWorkerToJobBlocklist(ctx, job, worker, "blender")
	require.NoError(t, err)
	err = db.AddWorkerToJobBlocklist(ctx, job, worker, "ffmpeg")
	require.NoError(t, err)

	// Clear the blocklist.
	err = db.ClearJobBlocklist(ctx, job)
	require.NoError(t, err)

	// Check that it is indeed empty.
	list, err := db.FetchJobBlocklist(ctx, job.UUID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestRemoveFromJobBlocklist(t *testing.T) {
	ctx, close, db, job, _ := jobTasksTestFixtures(t)
	defer close()

	// Add a worker and some entries to the block list.
	worker := createWorker(ctx, t, db)
	err := db.AddWorkerToJobBlocklist(ctx, job, worker, "blender")
	require.NoError(t, err)
	err = db.AddWorkerToJobBlocklist(ctx, job, worker, "ffmpeg")
	require.NoError(t, err)

	// Remove an entry.
	err = db.RemoveFromJobBlocklist(ctx, job.UUID, worker.UUID, "ffmpeg")
	require.NoError(t, err)

	// Check that the other entry is still there.
	list, err := db.FetchJobBlocklist(ctx, job.UUID)
	require.NoError(t, err)

	if assert.Len(t, list, 1) {
		entry := list[0]
		assert.Equal(t, worker.UUID, entry.WorkerUUID)
		assert.Equal(t, worker.Name, entry.WorkerName)
		assert.Equal(t, "blender", entry.TaskType)
	}
}

func TestWorkersLeftToRun(t *testing.T) {
	ctx, close, db, job, _ := jobTasksTestFixtures(t)
	defer close()

	// No workers.
	left, err := db.WorkersLeftToRun(ctx, job, "blender")
	require.NoError(t, err)
	assert.Empty(t, left)

	worker1 := createWorker(ctx, t, db)
	worker2 := createWorkerFrom(ctx, t, db, *worker1)

	// Create one worker tag. It will not be used by this job, but one of the
	// workers will be assigned to it. It can get this job's tasks, though.
	// Because the job is tagless, it can be run by all.
	tag1 := WorkerTag{UUID: "11157623-4b14-4801-bee2-271dddab6309", Name: "Tag 1"}
	require.NoError(t, db.CreateWorkerTag(ctx, &tag1))
	workerC1 := createWorker(ctx, t, db, func(w *Worker) {
		w.UUID = "c1c1c1c1-0000-1111-2222-333333333333"
		w.Tags = []*WorkerTag{&tag1}
	})

	uuidMap := func(workers ...*Worker) map[string]bool {
		theMap := map[string]bool{}
		for _, worker := range workers {
			theMap[worker.UUID] = true
		}
		return theMap
	}

	// Three workers, no blocklist.
	left, err = db.WorkersLeftToRun(ctx, job, "blender")
	require.NoError(t, err)
	assert.Equal(t, uuidMap(worker1, worker2, workerC1), left)

	// Two workers, one blocked.
	_ = db.AddWorkerToJobBlocklist(ctx, job, worker1, "blender")
	left, err = db.WorkersLeftToRun(ctx, job, "blender")
	require.NoError(t, err)
	assert.Equal(t, uuidMap(worker2, workerC1), left)

	// All workers blocked.
	_ = db.AddWorkerToJobBlocklist(ctx, job, worker2, "blender")
	_ = db.AddWorkerToJobBlocklist(ctx, job, workerC1, "blender")
	left, err = db.WorkersLeftToRun(ctx, job, "blender")
	require.NoError(t, err)
	assert.Empty(t, left)

	// Two workers, unknown job.
	fakeJob := Job{Model: Model{ID: 327}}
	left, err = db.WorkersLeftToRun(ctx, &fakeJob, "blender")
	require.NoError(t, err)
	assert.Equal(t, uuidMap(worker1, worker2, workerC1), left)
}

func TestWorkersLeftToRunWithTags(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	// Create tags.
	tag1 := WorkerTag{UUID: "11157623-4b14-4801-bee2-271dddab6309", Name: "Tag 1"}
	tag2 := WorkerTag{UUID: "22257623-4b14-4801-bee2-271dddab6309", Name: "Tag 2"}
	tag3 := WorkerTag{UUID: "33357623-4b14-4801-bee2-271dddab6309", Name: "Tag 3"}
	require.NoError(t, db.CreateWorkerTag(ctx, &tag1))
	require.NoError(t, db.CreateWorkerTag(ctx, &tag2))
	require.NoError(t, db.CreateWorkerTag(ctx, &tag3))

	// Create a job in tag1.
	authoredJob := createTestAuthoredJobWithTasks()
	authoredJob.WorkerTagUUID = tag1.UUID
	job := persistAuthoredJob(t, ctx, db, authoredJob)

	// Tags 1 + 3
	workerC13 := createWorker(ctx, t, db, func(w *Worker) {
		w.UUID = "c13c1313-0000-1111-2222-333333333333"
		w.Tags = []*WorkerTag{&tag1, &tag3}
	})
	// Tag 1
	workerC1 := createWorker(ctx, t, db, func(w *Worker) {
		w.UUID = "c1c1c1c1-0000-1111-2222-333333333333"
		w.Tags = []*WorkerTag{&tag1}
	})
	// Tag 2 worker, this one should never appear.
	createWorker(ctx, t, db, func(w *Worker) {
		w.UUID = "c2c2c2c2-0000-1111-2222-333333333333"
		w.Tags = []*WorkerTag{&tag2}
	})
	// No tags, so should be able to run only tagless jobs. Which is none
	// in this test.
	createWorker(ctx, t, db, func(w *Worker) {
		w.UUID = "00000000-0000-1111-2222-333333333333"
		w.Tags = nil
	})

	uuidMap := func(workers ...*Worker) map[string]bool {
		theMap := map[string]bool{}
		for _, worker := range workers {
			theMap[worker.UUID] = true
		}
		return theMap
	}

	// All Tag 1 workers, no blocklist.
	left, err := db.WorkersLeftToRun(ctx, job, "blender")
	require.NoError(t, err)
	assert.Equal(t, uuidMap(workerC13, workerC1), left)

	// One worker blocked, one worker remain.
	_ = db.AddWorkerToJobBlocklist(ctx, job, workerC1, "blender")
	left, err = db.WorkersLeftToRun(ctx, job, "blender")
	require.NoError(t, err)
	assert.Equal(t, uuidMap(workerC13), left)

	// All taged workers blocked.
	_ = db.AddWorkerToJobBlocklist(ctx, job, workerC13, "blender")
	left, err = db.WorkersLeftToRun(ctx, job, "blender")
	require.NoError(t, err)
	assert.Empty(t, left)
}

func TestCountTaskFailuresOfWorker(t *testing.T) {
	ctx, close, db, dbJob, authoredJob := jobTasksTestFixtures(t)
	defer close()

	task0, err := db.FetchTask(ctx, authoredJob.Tasks[0].UUID)
	require.NoError(t, err)
	task1, err := db.FetchTask(ctx, authoredJob.Tasks[1].UUID)
	require.NoError(t, err)
	task2, err := db.FetchTask(ctx, authoredJob.Tasks[2].UUID)
	require.NoError(t, err)

	// Sanity check on the test data.
	assert.Equal(t, "blender", task0.Type)
	assert.Equal(t, "blender", task1.Type)
	assert.Equal(t, "ffmpeg", task2.Type)

	worker1 := createWorker(ctx, t, db)
	worker2 := createWorkerFrom(ctx, t, db, *worker1)

	// Store some failures for different tasks
	_, _ = db.AddWorkerToTaskFailedList(ctx, task0, worker1)
	_, _ = db.AddWorkerToTaskFailedList(ctx, task1, worker1)
	_, _ = db.AddWorkerToTaskFailedList(ctx, task1, worker2)
	_, _ = db.AddWorkerToTaskFailedList(ctx, task2, worker1)

	// Multiple failures.
	numBlender1, err := db.CountTaskFailuresOfWorker(ctx, dbJob, worker1, "blender")
	require.NoError(t, err)
	assert.Equal(t, 2, numBlender1)

	// Single failure, but multiple tasks exist of this type.
	numBlender2, err := db.CountTaskFailuresOfWorker(ctx, dbJob, worker2, "blender")
	require.NoError(t, err)
	assert.Equal(t, 1, numBlender2)

	// Single failure, only one task of this type exists.
	numFFMpeg1, err := db.CountTaskFailuresOfWorker(ctx, dbJob, worker1, "ffmpeg")
	require.NoError(t, err)
	assert.Equal(t, 1, numFFMpeg1)

	// No failure.
	numFFMpeg2, err := db.CountTaskFailuresOfWorker(ctx, dbJob, worker2, "ffmpeg")
	require.NoError(t, err)
	assert.Equal(t, 0, numFFMpeg2)
}
