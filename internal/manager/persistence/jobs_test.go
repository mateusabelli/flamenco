// Package persistence provides the database interface for Flamenco Manager.
package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"projects.blender.org/studio/flamenco/internal/manager/job_compilers"
	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
	"projects.blender.org/studio/flamenco/internal/uuid"
	"projects.blender.org/studio/flamenco/pkg/api"
)

func TestStoreAuthoredJob(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(1 * time.Second)
	defer cancel()
	queries := db.queries()

	job := createTestAuthoredJobWithTasks()
	err := db.StoreAuthoredJob(ctx, job)
	require.NoError(t, err)

	fetchedJob, err := db.FetchJob(ctx, job.JobID)
	require.NoError(t, err)
	assert.NotNil(t, fetchedJob)

	// Test contents of fetched job
	assert.Equal(t, job.JobID, fetchedJob.UUID)
	assert.Equal(t, job.Name, fetchedJob.Name)
	assert.Equal(t, job.JobType, fetchedJob.JobType)
	assert.Equal(t, job.Priority, int(fetchedJob.Priority))
	assert.Equal(t, api.JobStatusUnderConstruction, fetchedJob.Status)
	assert.Equal(t, "", fetchedJob.StorageShamanCheckoutID)

	var parsedSettings map[string]interface{}
	assert.NoError(t, json.Unmarshal(fetchedJob.Settings, &parsedSettings))
	assert.EqualValues(t, map[string]interface{}(job.Settings), parsedSettings)

	var parsedMetadata map[string]string
	assert.NoError(t, json.Unmarshal(fetchedJob.Metadata, &parsedMetadata))
	assert.EqualValues(t, map[string]string(job.Metadata), parsedMetadata)

	// Fetch result of job.
	result, err := queries.FetchTasksOfJob(ctx, int64(fetchedJob.ID))
	require.NoError(t, err)

	if len(result) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(result))
	}

	// TODO: test task contents.
	assert.Equal(t, api.TaskStatusQueued, api.TaskStatus(result[0].Task.Status))
	assert.Equal(t, api.TaskStatusQueued, api.TaskStatus(result[1].Task.Status))
	assert.Equal(t, api.TaskStatusQueued, api.TaskStatus(result[2].Task.Status))
}

func TestStoreAuthoredJobWithShamanCheckoutID(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(1 * time.Second)
	defer cancel()

	job := createTestAuthoredJobWithTasks()
	job.Storage.ShamanCheckoutID = "één/twee"

	err := db.StoreAuthoredJob(ctx, job)
	require.NoError(t, err)

	fetchedJob, err := db.FetchJob(ctx, job.JobID)
	require.NoError(t, err)
	require.NotNil(t, fetchedJob)

	assert.Equal(t, job.Storage.ShamanCheckoutID, fetchedJob.StorageShamanCheckoutID)
}

func TestStoreAuthoredJobWithWorkerTag(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(1 * time.Second)
	defer cancel()

	workerTagUUID := "daa811ac-6861-4004-8748-7700aebc244c"
	require.NoError(t, db.CreateWorkerTag(ctx, &WorkerTag{
		UUID:        workerTagUUID,
		Name:        "🐈",
		Description: "Mrieuw",
	}))
	workerTag, err := db.FetchWorkerTag(ctx, workerTagUUID)
	require.NoError(t, err)

	job := createTestAuthoredJobWithTasks()
	job.WorkerTagUUID = workerTagUUID

	err = db.StoreAuthoredJob(ctx, job)
	require.NoError(t, err)

	fetchedJob, err := db.FetchJob(ctx, job.JobID)
	require.NoError(t, err)
	require.NotNil(t, fetchedJob)

	require.NotNil(t, fetchedJob.WorkerTagID)
	assert.Equal(t, fetchedJob.WorkerTagID.Int64, workerTag.ID)
	assert.True(t, fetchedJob.WorkerTagID.Valid)
}

func TestFetchTaskJobUUID(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(1 * time.Second)
	defer cancel()

	job := createTestAuthoredJobWithTasks()
	err := db.StoreAuthoredJob(ctx, job)
	require.NoError(t, err)

	jobUUID, err := db.FetchTaskJobUUID(ctx, job.Tasks[0].UUID)
	require.NoError(t, err)
	assert.Equal(t, job.JobID, jobUUID)
}

func TestSaveJobStorageInfo(t *testing.T) {
	// Test that saving job storage info doesn't count as "update".
	// This is necessary for `cmd/shaman-checkout-id-setter` to do its work quietly.
	ctx, cancel, db := persistenceTestFixtures(1 * time.Second)
	defer cancel()

	startTime := time.Date(2023, time.February, 7, 15, 0, 0, 0, time.UTC)
	mockNow := startTime
	db.nowfunc = func() time.Time { return mockNow }

	authoredJob := createTestAuthoredJobWithTasks()
	err := db.StoreAuthoredJob(ctx, authoredJob)
	require.NoError(t, err)

	dbJob, err := db.FetchJob(ctx, authoredJob.JobID)
	require.NoError(t, err)
	assert.NotNil(t, dbJob)
	assert.EqualValues(t, startTime, dbJob.UpdatedAt.Time)

	// Move the clock forward.
	updateTime := time.Date(2023, time.February, 7, 15, 10, 0, 0, time.UTC)
	mockNow = updateTime

	// Save the storage info.
	dbJob.StorageShamanCheckoutID = "shaman/checkout/id"
	require.NoError(t, db.SaveJobStorageInfo(ctx, dbJob))

	// Check that the UpdatedAt field wasn't touched.
	updatedJob, err := db.FetchJob(ctx, authoredJob.JobID)
	require.NoError(t, err)
	assert.Equal(t, startTime, updatedJob.UpdatedAt.Time, "SaveJobStorageInfo should not touch UpdatedAt")
}

func TestSaveJobPriority(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(1 * time.Second)
	defer cancel()

	// Create test job.
	authoredJob := createTestAuthoredJobWithTasks()
	err := db.StoreAuthoredJob(ctx, authoredJob)
	require.NoError(t, err)

	// Set a new priority.
	newPriority := int64(47)
	dbJob, err := db.FetchJob(ctx, authoredJob.JobID)
	require.NoError(t, err)
	require.NotEqual(t, newPriority, dbJob.Priority,
		"Initial priority should not be the same as what this test changes it to")
	dbJob.Priority = newPriority
	require.NoError(t, db.SaveJobPriority(ctx, dbJob))

	// Check the result.
	dbJob, err = db.FetchJob(ctx, authoredJob.JobID)
	require.NoError(t, err)
	assert.EqualValues(t, newPriority, dbJob.Priority)
}

func TestDeleteJob(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(1 * time.Second)
	defer cancel()
	queries := db.queries()

	authJob := createTestAuthoredJobWithTasks()
	authJob.Name = "Job to delete"
	persistAuthoredJob(t, ctx, db, authJob)

	otherJob := duplicateJobAndTasks(authJob)
	otherJob.Name = "The other job"
	otherJobTaskCount := int64(len(otherJob.Tasks))
	persistAuthoredJob(t, ctx, db, otherJob)

	// Delete the job.
	err := db.DeleteJob(ctx, authJob.JobID)
	require.NoError(t, err)

	// Test it cannot be found via the API.
	_, err = db.FetchJob(ctx, authJob.JobID)
	assert.ErrorIs(t, err, ErrJobNotFound, "deleted jobs should not be found")

	// Test that the job is really gone.
	numJobs, err := queries.Test_CountJobs(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), numJobs,
		"the job should have been deleted, and the other one should still be there")

	// Test that the tasks are gone too.
	numTasks, err := queries.Test_CountTasks(ctx)
	require.NoError(t, err)
	assert.Equal(t, otherJobTaskCount, numTasks,
		"tasks should have been deleted along with their job, and the other job's tasks should still be there")

	// Test that the correct job was deleted.
	dbOtherJob, err := db.FetchJob(ctx, otherJob.JobID)
	require.NoError(t, err, "the other job should still be there")
	assert.Equal(t, otherJob.Name, dbOtherJob.Name)

	// Test that all the remaining tasks belong to that particular job.
	tasksOfJob, err := queries.FetchTasksOfJob(ctx, int64(dbOtherJob.ID))
	require.NoError(t, err)
	assert.Equal(t, len(tasksOfJob), int(numTasks),
		"all remaining tasks should belong to the other job")
}

func TestFetchJobShamanCheckoutID(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(1 * time.Second)
	defer cancel()

	authJob := createTestAuthoredJobWithTasks()
	authJob.JobID = "e1a034cc-b709-45f5-b80f-9cf16511c678"
	authJob.Name = "Job to delete"
	authJob.Storage.ShamanCheckoutID = "some-✓out-id-string"
	persistAuthoredJob(t, ctx, db, authJob)

	{ // Test fetching a non-existing job.
		checkoutID, err := db.FetchJobShamanCheckoutID(ctx, "4cb20f0d-f1f6-4d56-8277-9b208a99fed0")
		assert.ErrorIs(t, err, ErrJobNotFound)
		assert.Equal(t, "", checkoutID)
	}

	{ // Test existing job.
		checkoutID, err := db.FetchJobShamanCheckoutID(ctx, authJob.JobID)
		require.NoError(t, err)
		assert.Equal(t, authJob.Storage.ShamanCheckoutID, checkoutID)
	}
}

func TestDeleteJobWithoutFK(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(1 * time.Second)
	defer cancel()

	authJob := createTestAuthoredJobWithTasks()
	authJob.Name = "Job to delete"
	persistAuthoredJob(t, ctx, db, authJob)

	require.NoError(t, db.pragmaForeignKeys(ctx, false))

	err := db.DeleteJob(ctx, authJob.JobID)
	require.ErrorIs(t, err, ErrDeletingWithoutFK)

	// Test the deletion did not happen.
	_, err = db.FetchJob(ctx, authJob.JobID)
	require.NoError(t, err, "job should not have been deleted")
}

func TestRequestJobDeletion(t *testing.T) {
	ctx, close, db, job1, authoredJob1 := jobTasksTestFixtures(t)
	defer close()

	// Create another job, to see it's not touched by deleting the first one.
	authoredJob2 := duplicateJobAndTasks(authoredJob1)
	persistAuthoredJob(t, ctx, db, authoredJob2)

	mockNow := time.Now().UTC()
	db.nowfunc = func() time.Time { return mockNow }

	err := db.RequestJobDeletion(ctx, job1)
	require.NoError(t, err)
	assert.True(t, job1.DeleteRequested())
	assert.True(t, job1.DeleteRequestedAt.Valid)
	assert.Equal(t, job1.DeleteRequestedAt.Time, mockNow)

	dbJob1, err := db.FetchJob(ctx, job1.UUID)
	require.NoError(t, err)
	assert.True(t, job1.DeleteRequested())
	assert.True(t, dbJob1.DeleteRequestedAt.Valid)
	assert.WithinDuration(t, mockNow, dbJob1.DeleteRequestedAt.Time, time.Second)

	// Other jobs shouldn't be touched.
	dbJob2, err := db.FetchJob(ctx, authoredJob2.JobID)
	require.NoError(t, err)
	assert.False(t, dbJob2.DeleteRequested())
	assert.False(t, dbJob2.DeleteRequestedAt.Valid)
}

func TestRequestJobMassDeletion(t *testing.T) {
	// This is a fresh job, that shouldn't be touched by the mass deletion.
	ctx, close, db, job1, authoredJob1 := jobTasksTestFixtures(t)
	defer close()

	realNowFunc := db.nowfunc
	now := db.now()

	// Ensure different jobs get different timestamps.
	db.nowfunc = func() time.Time { return now.Add(-3 * time.Second) }
	authoredJob2 := duplicateJobAndTasks(authoredJob1)
	job2 := persistAuthoredJob(t, ctx, db, authoredJob2)

	db.nowfunc = func() time.Time { return now.Add(-4 * time.Second) }
	authoredJob3 := duplicateJobAndTasks(authoredJob1)
	job3 := persistAuthoredJob(t, ctx, db, authoredJob3)

	db.nowfunc = func() time.Time { return now.Add(-5 * time.Second) }
	authoredJob4 := duplicateJobAndTasks(authoredJob1)
	job4 := persistAuthoredJob(t, ctx, db, authoredJob4)

	// Request that "job3 and older" gets deleted.
	timeOfDeleteRequest := realNowFunc()
	db.nowfunc = func() time.Time { return timeOfDeleteRequest }
	uuids, err := db.RequestJobMassDeletion(ctx, job3.UpdatedAt.Time)
	require.NoError(t, err)

	db.nowfunc = realNowFunc

	// Only jobs 3 and 4 should be updated.
	assert.Equal(t, []string{job3.UUID, job4.UUID}, uuids)

	// All the jobs should still exist.
	job1, err = db.FetchJob(ctx, job1.UUID)
	require.NoError(t, err)
	job2, err = db.FetchJob(ctx, job2.UUID)
	require.NoError(t, err)
	job3, err = db.FetchJob(ctx, job3.UUID)
	require.NoError(t, err)
	job4, err = db.FetchJob(ctx, job4.UUID)
	require.NoError(t, err)

	// Jobs 3 and 4 should have been marked for deletion, the rest should be untouched.
	assert.False(t, job1.DeleteRequested())
	assert.False(t, job2.DeleteRequested())
	assert.True(t, job3.DeleteRequested())
	assert.True(t, job4.DeleteRequested())

	assert.Equal(t, timeOfDeleteRequest, job3.DeleteRequestedAt.Time)
	assert.Equal(t, timeOfDeleteRequest, job4.DeleteRequestedAt.Time)
}

func TestRequestJobMassDeletion_noJobsFound(t *testing.T) {
	ctx, close, db, job, _ := jobTasksTestFixtures(t)
	defer close()

	// Request deletion with a timestamp that doesn't match any jobs.
	now := db.now()
	uuids, err := db.RequestJobMassDeletion(ctx, now.Add(-24*time.Hour))
	assert.ErrorIs(t, err, ErrJobNotFound)
	assert.Zero(t, uuids)

	// The job shouldn't have been touched.
	job, err = db.FetchJob(ctx, job.UUID)
	require.NoError(t, err)
	assert.False(t, job.DeleteRequested())
}

func TestFetchJobsDeletionRequested(t *testing.T) {
	ctx, close, db, job1, authoredJob1 := jobTasksTestFixtures(t)
	defer close()

	now := time.Now()
	db.nowfunc = func() time.Time { return now }

	authoredJob2 := duplicateJobAndTasks(authoredJob1)
	job2 := persistAuthoredJob(t, ctx, db, authoredJob2)
	authoredJob3 := duplicateJobAndTasks(authoredJob1)
	job3 := persistAuthoredJob(t, ctx, db, authoredJob3)
	authoredJob4 := duplicateJobAndTasks(authoredJob1)
	persistAuthoredJob(t, ctx, db, authoredJob4)

	// Ensure different requests get different timestamps,
	// out of chronological order.
	timestamps := []time.Time{
		// timestamps for 'delete requested at'.
		now.Add(-3 * time.Second),
		now.Add(-1 * time.Second),
		now.Add(-5 * time.Second),
	}
	currentTimestampIndex := 0
	db.nowfunc = func() time.Time {
		now := timestamps[currentTimestampIndex]
		currentTimestampIndex++
		return now
	}

	err := db.RequestJobDeletion(ctx, job1)
	require.NoError(t, err)
	err = db.RequestJobDeletion(ctx, job2)
	require.NoError(t, err)
	err = db.RequestJobDeletion(ctx, job3)
	require.NoError(t, err)

	actualUUIDs, err := db.FetchJobsDeletionRequested(ctx)
	require.NoError(t, err)
	assert.Len(t, actualUUIDs, 3, "3 out of 4 jobs were marked for deletion")

	// Expect UUIDs in chronological order of deletion requests, so that the
	// oldest request is handled first.
	expectUUIDs := []string{job3.UUID, job1.UUID, job2.UUID}
	assert.Equal(t, expectUUIDs, actualUUIDs)
}

func TestJobHasTasksInStatus(t *testing.T) {
	ctx, close, db, job, _ := jobTasksTestFixtures(t)
	defer close()

	hasTasks, err := db.JobHasTasksInStatus(ctx, job, api.TaskStatusQueued)
	require.NoError(t, err)
	assert.True(t, hasTasks, "expected freshly-created job to have queued tasks")

	hasTasks, err = db.JobHasTasksInStatus(ctx, job, api.TaskStatusActive)
	require.NoError(t, err)
	assert.False(t, hasTasks, "expected freshly-created job to have no active tasks")
}

func TestCountTasksOfJobInStatus(t *testing.T) {
	ctx, close, db, job, authoredJob := jobTasksTestFixtures(t)
	defer close()

	numQueued, numTotal, err := db.CountTasksOfJobInStatus(ctx, job, api.TaskStatusQueued)
	require.NoError(t, err)
	assert.Equal(t, 3, numQueued)
	assert.Equal(t, 3, numTotal)

	// Make one taskJobWorker failed.
	taskJobWorker, err := db.FetchTask(ctx, authoredJob.Tasks[0].UUID)
	require.NoError(t, err)
	taskJobWorker.Task.Status = api.TaskStatusFailed
	require.NoError(t, db.SaveTask(ctx, &taskJobWorker.Task))

	numQueued, numTotal, err = db.CountTasksOfJobInStatus(ctx, job, api.TaskStatusQueued)
	require.NoError(t, err)
	assert.Equal(t, 2, numQueued)
	assert.Equal(t, 3, numTotal)

	numFailed, numTotal, err := db.CountTasksOfJobInStatus(ctx, job, api.TaskStatusFailed)
	require.NoError(t, err)
	assert.Equal(t, 1, numFailed)
	assert.Equal(t, 3, numTotal)

	numActive, numTotal, err := db.CountTasksOfJobInStatus(ctx, job, api.TaskStatusActive)
	require.NoError(t, err)
	assert.Equal(t, 0, numActive)
	assert.Equal(t, 3, numTotal)

	numCounted, numTotal, err := db.CountTasksOfJobInStatus(ctx, job,
		api.TaskStatusFailed, api.TaskStatusQueued)
	require.NoError(t, err)
	assert.Equal(t, 3, numCounted)
	assert.Equal(t, 3, numTotal)
}

func TestCheckIfJobsHoldLargeNumOfTasks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
	numtasks := 3500
	ctx, close, db, job, _ := jobTasksTestFixturesWithTaskNum(t, numtasks)
	defer close()

	numQueued, numTotal, err := db.CountTasksOfJobInStatus(ctx, job, api.TaskStatusQueued)
	require.NoError(t, err)
	assert.Equal(t, numtasks, numQueued)
	assert.Equal(t, numtasks, numTotal)

}

func TestFetchJobsInStatus(t *testing.T) {
	ctx, close, db, job1, _ := jobTasksTestFixtures(t)
	defer close()

	ajob2 := createTestAuthoredJob("1f08e20b-ce24-41c2-b237-36120bd69fc6")
	ajob3 := createTestAuthoredJob("3ac2dbb4-0c34-410e-ad3b-652e6d7e65a5")
	job2 := persistAuthoredJob(t, ctx, db, ajob2)
	job3 := persistAuthoredJob(t, ctx, db, ajob3)

	// Sanity check
	if !assert.Equal(t, api.JobStatusUnderConstruction, job1.Status) {
		return
	}

	// Query single status
	jobs, err := db.FetchJobsInStatus(ctx, api.JobStatusUnderConstruction)
	require.NoError(t, err)
	assert.Equal(t, []*Job{job1, job2, job3}, jobs)

	// Query two statuses, where only one matches all jobs.
	jobs, err = db.FetchJobsInStatus(ctx, api.JobStatusCanceled, api.JobStatusUnderConstruction)
	require.NoError(t, err)
	assert.Equal(t, []*Job{job1, job2, job3}, jobs)

	// Update a job status, query for two of the three used statuses.
	job1.Status = api.JobStatusQueued
	require.NoError(t, db.SaveJobStatus(ctx, job1))
	job2.Status = api.JobStatusRequeueing
	require.NoError(t, db.SaveJobStatus(ctx, job2))

	jobs, err = db.FetchJobsInStatus(ctx, api.JobStatusQueued, api.JobStatusUnderConstruction)
	require.NoError(t, err)
	if assert.Len(t, jobs, 2) {
		assert.Equal(t, job1.UUID, jobs[0].UUID)
		assert.Equal(t, job3.UUID, jobs[1].UUID)
	}
}

func TestFetchTasksOfJobInStatus(t *testing.T) {
	ctx, close, db, job, authoredJob := jobTasksTestFixtures(t)
	defer close()

	allTasks, err := db.FetchTasksOfJob(ctx, job)
	require.NoError(t, err)
	assert.Equal(t, job.UUID, allTasks[0].JobUUID, "FetchTasksOfJob should set job UUID")

	tasksJobsWorkers, err := db.FetchTasksOfJobInStatus(ctx, job, api.TaskStatusQueued)
	require.NoError(t, err)
	assert.Equal(t, allTasks, tasksJobsWorkers)
	assert.Equal(t, job.UUID, tasksJobsWorkers[0].JobUUID, "FetchTasksOfJobInStatus should set job UUID")

	// Make one taskJobWorker failed.
	taskJobWorker, err := db.FetchTask(ctx, authoredJob.Tasks[0].UUID)
	require.NoError(t, err)
	taskJobWorker.Task.Status = api.TaskStatusFailed
	require.NoError(t, db.SaveTask(ctx, &taskJobWorker.Task))

	tasksJobsWorkers, err = db.FetchTasksOfJobInStatus(ctx, job, api.TaskStatusQueued)
	require.NoError(t, err)
	assert.Equal(t, []TaskJobWorker{allTasks[1], allTasks[2]}, tasksJobsWorkers)

	// Check the failed task. This cannot directly compare to `allTasks[0]`
	// because saving the task above changed some of its fields.
	tasksJobsWorkers, err = db.FetchTasksOfJobInStatus(ctx, job, api.TaskStatusFailed)
	require.NoError(t, err)
	assert.Len(t, tasksJobsWorkers, 1)
	assert.Equal(t, allTasks[0].Task.ID, tasksJobsWorkers[0].Task.ID)

	tasksJobsWorkers, err = db.FetchTasksOfJobInStatus(ctx, job, api.TaskStatusActive)
	require.NoError(t, err)
	assert.Empty(t, tasksJobsWorkers)
}

func TestSaveTaskActivity(t *testing.T) {
	ctx, close, db, _, authoredJob := jobTasksTestFixtures(t)
	defer close()

	taskUUID := authoredJob.Tasks[0].UUID
	taskJobWorker, err := db.FetchTask(ctx, taskUUID)
	require.NoError(t, err)

	task := taskJobWorker.Task
	require.Equal(t, api.TaskStatusQueued, task.Status)

	task.Activity = "Somebody ran a ünit test"
	task.Status = api.TaskStatusPaused // Should not be saved.
	require.NoError(t, db.SaveTaskActivity(ctx, &task))

	dbTaskJobWorker, err := db.FetchTask(ctx, taskUUID)
	require.NoError(t, err)

	dbTask := dbTaskJobWorker.Task
	require.Equal(t, "Somebody ran a ünit test", dbTask.Activity)
	require.Equal(t, api.TaskStatusQueued, dbTask.Status,
		"SaveTaskActivity() should not save the task status")
}

func TestTaskAssignToWorker(t *testing.T) {
	ctx, close, db, _, authoredJob := jobTasksTestFixtures(t)
	defer close()

	taskJobWorker, err := db.FetchTask(ctx, authoredJob.Tasks[1].UUID)
	require.NoError(t, err)
	assert.Zero(t, taskJobWorker.WorkerUUID)
	assert.Equal(t, authoredJob.JobID, taskJobWorker.JobUUID)

	w := createWorker(ctx, t, db)
	require.NoError(t, db.TaskAssignToWorker(ctx, &taskJobWorker.Task, w))
	assert.Equal(t,
		sql.NullInt64{Int64: w.ID, Valid: true},
		taskJobWorker.Task.WorkerID)
}

func TestFetchTasksOfWorkerInStatus(t *testing.T) {
	ctx, close, db, _, authoredJob := jobTasksTestFixtures(t)
	defer close()

	taskJobWorker, err := db.FetchTask(ctx, authoredJob.Tasks[1].UUID)
	task := taskJobWorker.Task
	require.NoError(t, err)

	w := createWorker(ctx, t, db)
	require.NoError(t, db.TaskAssignToWorker(ctx, &task, w))

	tasksJobsWorkers, err := db.FetchTasksOfWorkerInStatus(ctx, w, task.Status)
	require.NoError(t, err)
	require.Len(t, tasksJobsWorkers, 1, "worker should have one task in status %q", task.Status)
	assert.Equal(t, task.ID, tasksJobsWorkers[0].Task.ID)
	assert.Equal(t, task.UUID, tasksJobsWorkers[0].Task.UUID)
	assert.Equal(t, authoredJob.JobID, tasksJobsWorkers[0].JobUUID)

	require.NotEqual(t, api.TaskStatusCanceled, task.Status)
	tasksJobsWorkers, err = db.FetchTasksOfWorkerInStatus(ctx, w, api.TaskStatusCanceled)
	require.NoError(t, err)
	assert.Empty(t, tasksJobsWorkers, "worker should have no task in status %q", api.TaskStatusCanceled)
}

func TestFetchTasksOfWorkerInStatusOfJob(t *testing.T) {
	ctx, close, db, dbJob, authoredJob := jobTasksTestFixtures(t)
	defer close()

	// Create multiple Workers, to test the function doesn't return tasks from
	// other Workers.
	worker := createWorker(ctx, t, db, func(worker *Worker) {
		worker.UUID = "43300628-5f3b-4724-ab30-9821af8bda86"
	})
	otherWorker := createWorker(ctx, t, db, func(worker *Worker) {
		worker.UUID = "2327350f-75ec-4b0e-bd28-31a7b045c85c"
	})

	// Create another job, to make sure the function under test doesn't return
	// tasks from other jobs.
	otherJob := duplicateJobAndTasks(authoredJob)
	otherJob.Name = "The other job"
	persistAuthoredJob(t, ctx, db, otherJob)

	// Assign a task from each job to each Worker.
	// Also double-check the test precondition that all tasks have the same status.
	{ // Job / Worker.
		taskJobWorker1, err := db.FetchTask(ctx, authoredJob.Tasks[1].UUID)
		require.NoError(t, err)
		require.NoError(t, db.TaskAssignToWorker(ctx, &taskJobWorker1.Task, worker))
		require.Equal(t, taskJobWorker1.Task.Status, api.TaskStatusQueued)

		taskJobWorker2, err := db.FetchTask(ctx, authoredJob.Tasks[0].UUID)
		require.NoError(t, err)
		require.NoError(t, db.TaskAssignToWorker(ctx, &taskJobWorker2.Task, worker))
		require.Equal(t, taskJobWorker2.Task.Status, api.TaskStatusQueued)
	}
	{ // Job / Other Worker.
		taskJobWorker, err := db.FetchTask(ctx, authoredJob.Tasks[2].UUID)
		require.NoError(t, err)
		require.NoError(t, db.TaskAssignToWorker(ctx, &taskJobWorker.Task, otherWorker))
		require.Equal(t, taskJobWorker.Task.Status, api.TaskStatusQueued)
	}
	{ // Other Job / Worker.
		taskJobWorker, err := db.FetchTask(ctx, otherJob.Tasks[1].UUID)
		require.NoError(t, err)
		require.NoError(t, db.TaskAssignToWorker(ctx, &taskJobWorker.Task, worker))
		require.Equal(t, taskJobWorker.Task.Status, api.TaskStatusQueued)
	}
	{ // Other Job / Other Worker.
		taskJobWorker, err := db.FetchTask(ctx, otherJob.Tasks[2].UUID)
		require.NoError(t, err)
		require.NoError(t, db.TaskAssignToWorker(ctx, &taskJobWorker.Task, otherWorker))
		require.Equal(t, taskJobWorker.Task.Status, api.TaskStatusQueued)
	}

	{ // Test active tasks, should be none.
		tasks, err := db.FetchTasksOfWorkerInStatusOfJob(ctx, worker, api.TaskStatusActive, dbJob.UUID)
		require.NoError(t, err)
		require.Len(t, tasks, 0)
	}
	{ // Test queued tasks, should be two.
		tasks, err := db.FetchTasksOfWorkerInStatusOfJob(ctx, worker, api.TaskStatusQueued, dbJob.UUID)
		require.NoError(t, err)
		require.Len(t, tasks, 2)
		assert.Equal(t, authoredJob.Tasks[0].UUID, tasks[0].UUID)
		assert.Equal(t, authoredJob.Tasks[1].UUID, tasks[1].UUID)
	}
	{ // Test queued tasks for worker without tasks, should be none.
		worker := createWorker(ctx, t, db, func(worker *Worker) {
			worker.UUID = "6534a1d4-f58e-4f2c-8925-4b2cd6caac22"
		})
		tasks, err := db.FetchTasksOfWorkerInStatusOfJob(ctx, worker, api.TaskStatusQueued, dbJob.UUID)
		require.NoError(t, err)
		require.Len(t, tasks, 0)
	}
}

func TestUpdateJobsTaskStepCounts(t *testing.T) {
	ctx, close, db, dbJob, authoredJob := jobTasksTestFixtures(t)
	defer close()

	taskJobWorker0, _ := db.FetchTask(ctx, authoredJob.Tasks[0].UUID)
	taskJobWorker1, _ := db.FetchTask(ctx, authoredJob.Tasks[1].UUID)
	taskJobWorker2, _ := db.FetchTask(ctx, authoredJob.Tasks[2].UUID)
	task0 := taskJobWorker0.Task
	task1 := taskJobWorker1.Task
	task2 := taskJobWorker2.Task

	task0.Status = "completed" // Should get max step count.
	task0.StepsTotal = 3
	task0.StepsCompleted = 2
	task1.Status = "active" // Shouldn't touch step count
	task1.StepsTotal = 3
	task1.StepsCompleted = 2
	task2.Status = "queued" // Should get zero step count.
	task2.StepsTotal = 3
	task2.StepsCompleted = 2

	require.NoError(t, db.SaveTask(ctx, &task0))
	require.NoError(t, db.SaveTask(ctx, &task1))
	require.NoError(t, db.SaveTask(ctx, &task2))

	require.NoError(t, db.UpdateJobsTaskStepCounts(ctx, dbJob.ID))

	// Check the database.
	taskJobWorker0, _ = db.FetchTask(ctx, task0.UUID)
	taskJobWorker1, _ = db.FetchTask(ctx, task1.UUID)
	taskJobWorker2, _ = db.FetchTask(ctx, task2.UUID)

	assert.Equal(t, int64(3), taskJobWorker0.Task.StepsCompleted)
	assert.Equal(t, int64(2), taskJobWorker1.Task.StepsCompleted)
	assert.Equal(t, int64(0), taskJobWorker2.Task.StepsCompleted)
}

func TestTaskTouchedByWorker(t *testing.T) {
	ctx, close, db, _, authoredJob := jobTasksTestFixtures(t)
	defer close()

	taskJobWorker, err := db.FetchTask(ctx, authoredJob.Tasks[1].UUID)
	task := taskJobWorker.Task
	require.NoError(t, err)
	assert.Zero(t, task.LastTouchedAt)

	now := db.now()
	err = db.TaskTouchedByWorker(ctx, task.UUID)
	require.NoError(t, err)

	// Test the task as it is in the database.
	dbTaskJobWorker, err := db.FetchTask(ctx, task.UUID)
	require.NoError(t, err)
	assert.True(t, dbTaskJobWorker.Task.LastTouchedAt.Valid)
	assert.WithinDuration(t, now, dbTaskJobWorker.Task.LastTouchedAt.Time, time.Second)
}

func TestAddWorkerToTaskFailedList(t *testing.T) {
	ctx, close, db, _, authoredJob := jobTasksTestFixtures(t)
	defer close()

	taskJobWorker, err := db.FetchTask(ctx, authoredJob.Tasks[1].UUID)
	require.NoError(t, err)
	task := taskJobWorker.Task

	worker1 := createWorker(ctx, t, db)

	// Create another worker, using the 1st as template:
	newWorker := *worker1
	newWorker.ID = 0
	newWorker.UUID = "89ed2b02-b51b-4cd4-b44a-4a1c8d01db85"
	newWorker.Name = "Worker 2"
	require.NoError(t, db.CreateWorker(ctx, &newWorker))
	worker2, err := db.FetchWorker(ctx, newWorker.UUID)
	require.NoError(t, err)

	// First failure should be registered just fine.
	numFailed, err := db.AddWorkerToTaskFailedList(ctx, &task, worker1)
	require.NoError(t, err)
	assert.Equal(t, 1, numFailed)

	// Calling again should be a no-op and not cause any errors.
	numFailed, err = db.AddWorkerToTaskFailedList(ctx, &task, worker1)
	require.NoError(t, err)
	assert.Equal(t, 1, numFailed)

	// Another worker should be able to fail this task as well.
	numFailed, err = db.AddWorkerToTaskFailedList(ctx, &task, worker2)
	require.NoError(t, err)
	assert.Equal(t, 2, numFailed)

	// Deleting the task should also delete the failures.
	require.NoError(t, db.DeleteJob(ctx, authoredJob.JobID))
	assert.Zero(t, countTaskFailures(ctx, db))
}

func TestClearFailureListOfTask(t *testing.T) {
	ctx, close, db, _, authoredJob := jobTasksTestFixtures(t)
	defer close()
	queries := db.queries()

	taskJobWorker1, _ := db.FetchTask(ctx, authoredJob.Tasks[1].UUID)
	taskJobWorker2, _ := db.FetchTask(ctx, authoredJob.Tasks[2].UUID)
	task1 := taskJobWorker1.Task
	task2 := taskJobWorker2.Task

	worker1 := createWorker(ctx, t, db)

	// Create another worker, using the 1st as template:
	newWorker := *worker1
	newWorker.ID = 0
	newWorker.UUID = "89ed2b02-b51b-4cd4-b44a-4a1c8d01db85"
	newWorker.Name = "Worker 2"
	require.NoError(t, db.CreateWorker(ctx, &newWorker))
	worker2, err := db.FetchWorker(ctx, newWorker.UUID)
	require.NoError(t, err)

	// Store some failures for different tasks.
	_, _ = db.AddWorkerToTaskFailedList(ctx, &task1, worker1)
	_, _ = db.AddWorkerToTaskFailedList(ctx, &task1, worker2)
	_, _ = db.AddWorkerToTaskFailedList(ctx, &task2, worker1)

	// Clearing should just update this one task.
	require.NoError(t, db.ClearFailureListOfTask(ctx, &task1))
	failures, err := queries.Test_FetchTaskFailures(ctx)
	require.NoError(t, err)
	if assert.Len(t, failures, 1) {
		assert.Equal(t, int64(task2.ID), failures[0].TaskID)
		assert.Equal(t, int64(worker1.ID), failures[0].WorkerID)
	}
}

func TestClearFailureListOfJob(t *testing.T) {
	ctx, close, db, dbJob1, authoredJob1 := jobTasksTestFixtures(t)
	defer close()
	queries := db.queries()

	// Construct a cloned version of the job.
	authoredJob2 := duplicateJobAndTasks(authoredJob1)
	persistAuthoredJob(t, ctx, db, authoredJob2)

	taskJobWorker1_1, _ := db.FetchTask(ctx, authoredJob1.Tasks[1].UUID)
	taskJobWorker1_2, _ := db.FetchTask(ctx, authoredJob1.Tasks[2].UUID)
	taskJobWorker2_1, _ := db.FetchTask(ctx, authoredJob2.Tasks[1].UUID)
	task1_1 := taskJobWorker1_1.Task
	task1_2 := taskJobWorker1_2.Task
	task2_1 := taskJobWorker2_1.Task

	worker1 := createWorker(ctx, t, db)
	worker2 := createWorkerFrom(ctx, t, db, *worker1)

	// Store some failures for different tasks and jobs
	_, _ = db.AddWorkerToTaskFailedList(ctx, &task1_1, worker1)
	_, _ = db.AddWorkerToTaskFailedList(ctx, &task1_1, worker2)
	_, _ = db.AddWorkerToTaskFailedList(ctx, &task1_2, worker1)
	_, _ = db.AddWorkerToTaskFailedList(ctx, &task2_1, worker1)
	_, _ = db.AddWorkerToTaskFailedList(ctx, &task2_1, worker2)

	// Sanity check: there should be 5 failures registered now.
	assert.Equal(t, 5, countTaskFailures(ctx, db))

	// Clearing should be limited to the given job.
	require.NoError(t, db.ClearFailureListOfJob(ctx, dbJob1))
	failures, err := queries.Test_FetchTaskFailures(ctx)
	require.NoError(t, err)
	if assert.Len(t, failures, 2) {
		assert.Equal(t, int64(task2_1.ID), failures[0].TaskID)
		assert.Equal(t, int64(worker1.ID), failures[0].WorkerID)
		assert.Equal(t, int64(task2_1.ID), failures[1].TaskID)
		assert.Equal(t, int64(worker2.ID), failures[1].WorkerID)
	}
}

func TestFetchTaskFailureList(t *testing.T) {
	ctx, close, db, _, authoredJob1 := jobTasksTestFixtures(t)
	defer close()

	// Test with non-existing task.
	fakeTask := Task{ID: 327}
	failures, err := db.FetchTaskFailureList(ctx, &fakeTask)
	require.NoError(t, err)
	assert.Empty(t, failures)

	taskJobWorker1_1, _ := db.FetchTask(ctx, authoredJob1.Tasks[1].UUID)
	taskJobWorker1_2, _ := db.FetchTask(ctx, authoredJob1.Tasks[2].UUID)
	task1_1 := taskJobWorker1_1.Task
	task1_2 := taskJobWorker1_2.Task

	// Test without failures.
	failures, err = db.FetchTaskFailureList(ctx, &task1_1)
	require.NoError(t, err)
	assert.Empty(t, failures)

	worker1 := createWorker(ctx, t, db)
	worker2 := createWorkerFrom(ctx, t, db, *worker1)

	// Store some failures for different tasks and jobs
	_, _ = db.AddWorkerToTaskFailedList(ctx, &task1_1, worker1)
	_, _ = db.AddWorkerToTaskFailedList(ctx, &task1_1, worker2)
	_, _ = db.AddWorkerToTaskFailedList(ctx, &task1_2, worker1)

	// Fetch one task's failure list.
	failures, err = db.FetchTaskFailureList(ctx, &task1_1)
	require.NoError(t, err)

	if assert.Len(t, failures, 2) {
		assert.Equal(t, worker1.UUID, failures[0].UUID)
		assert.Equal(t, worker1.Name, failures[0].Name)
		assert.Equal(t, worker1.Address, failures[0].Address)

		assert.Equal(t, worker2.UUID, failures[1].UUID)
		assert.Equal(t, worker2.Name, failures[1].Name)
		assert.Equal(t, worker2.Address, failures[1].Address)
	}
}

func createTestAuthoredJobWithTasks() job_compilers.AuthoredJob {
	task1 := job_compilers.AuthoredTask{
		Name: "render-1-3",
		Type: "blender",
		UUID: "db1f5481-4ef5-4084-8571-8460c547ecaa",
		Commands: []job_compilers.AuthoredCommand{
			{
				Name: "blender-render",
				Parameters: job_compilers.AuthoredCommandParameters{
					"exe":       "{blender}",
					"blendfile": "/path/to/file.blend",
					"args": []interface{}{
						"--render-output", "/path/to/output/######.png",
						"--render-format", "PNG",
						"--render-frame", "1-3",
					},
				}},
		},
	}

	task2 := task1
	task2.Name = "render-4-6"
	task2.UUID = "d75ac779-151b-4bc2-b8f1-d153a9c4ac69"
	task2.Commands[0].Parameters["frames"] = "4-6"

	task3 := job_compilers.AuthoredTask{
		Name: "preview-video",
		Type: "ffmpeg",
		UUID: "4915fb05-72f5-463e-a2f4-7efdb2584a1e",
		Commands: []job_compilers.AuthoredCommand{
			{
				Name: "merge-frames-to-video",
				Parameters: job_compilers.AuthoredCommandParameters{
					"images":       "/path/to/output/######.png",
					"output":       "/path/to/output/preview.mkv",
					"ffmpegParams": "-c:v hevc -crf 31",
				}},
		},
		Dependencies: []*job_compilers.AuthoredTask{&task1, &task2},
	}

	return createTestAuthoredJob("263fd47e-b9f8-4637-b726-fd7e47ecfdae", task1, task2, task3)
}

func createTestAuthoredJobWithNumTasks(numTasks int) job_compilers.AuthoredJob {
	//Generates all of the render jobs
	prevtasks := make([]*job_compilers.AuthoredTask, 0)
	for i := 0; i < numTasks-1; i++ {
		currtask := job_compilers.AuthoredTask{
			Name:     "render-" + fmt.Sprintf("%d", i),
			Type:     "blender-render",
			UUID:     uuid.New(),
			Commands: []job_compilers.AuthoredCommand{},
		}
		prevtasks = append(prevtasks, &currtask)
	}
	//	Generates the preview video command with Dependencies
	videoJob := job_compilers.AuthoredTask{
		Name:         "preview-video",
		Type:         "ffmpeg",
		UUID:         uuid.New(),
		Commands:     []job_compilers.AuthoredCommand{},
		Dependencies: prevtasks,
	}
	//	convert pointers to values and generate job
	taskvalues := make([]job_compilers.AuthoredTask, len(prevtasks))
	for i, ptr := range prevtasks {
		taskvalues[i] = *ptr
	}
	taskvalues = append(taskvalues, videoJob)
	return createTestAuthoredJob(uuid.New(), taskvalues...)

}

func createTestAuthoredJob(jobID string, tasks ...job_compilers.AuthoredTask) job_compilers.AuthoredJob {
	job := job_compilers.AuthoredJob{
		JobID:    jobID,
		Name:     "Test job",
		Status:   api.JobStatusUnderConstruction,
		Priority: 50,
		Settings: job_compilers.JobSettings{
			"frames":     "1-6",
			"chunk_size": 3.0, // The roundtrip to JSON in the database can make this a float.
		},
		Metadata: job_compilers.JobMetadata{
			"author":  "Sybren",
			"project": "Sprite Fright",
		},
		Tasks: tasks,
	}

	return job
}

func persistAuthoredJob(t require.TestingT, ctx context.Context, db *DB, authoredJob job_compilers.AuthoredJob) *Job {
	err := db.StoreAuthoredJob(ctx, authoredJob)
	require.NoError(t, err, "error storing authored job in DB")

	dbJob, err := db.FetchJob(ctx, authoredJob.JobID)
	require.NoError(t, err, "error fetching job from DB")
	require.NotNil(t, dbJob, "nil job obtained from DB but with no error!")

	return dbJob
}

// duplicateJobAndTasks constructs a copy of the given job and its tasks, ensuring new UUIDs.
// Does NOT copy settings, metadata, or commands. Just for testing with more than one job in the database.
func duplicateJobAndTasks(job job_compilers.AuthoredJob) job_compilers.AuthoredJob {
	// The function call already made a new AuthoredJob copy.
	// This function just needs to make the tasks are duplicated, make UUIDs
	// unique, and ensure that task pointers are pointing to the copy.

	// Duplicate task arrays.
	tasks := job.Tasks
	job.Tasks = []job_compilers.AuthoredTask{}
	job.Tasks = append(job.Tasks, tasks...)

	// Construct a mapping from old UUID to pointer-to-new-task
	taskPtrs := map[string]*job_compilers.AuthoredTask{}
	for idx := range job.Tasks {
		taskPtrs[job.Tasks[idx].UUID] = &job.Tasks[idx]
	}

	// Go over all task dependencies, as those are stored as pointers, and update them.
	for taskIdx := range job.Tasks {
		newDeps := make([]*job_compilers.AuthoredTask, len(job.Tasks[taskIdx].Dependencies))
		for depIdxs, oldTaskPtr := range job.Tasks[taskIdx].Dependencies {
			depUUID := oldTaskPtr.UUID
			newDeps[depIdxs] = taskPtrs[depUUID]
		}
		job.Tasks[taskIdx].Dependencies = newDeps
	}

	// Assign new UUIDs to the job & tasks.
	job.JobID = uuid.New()
	for idx := range job.Tasks {
		job.Tasks[idx].UUID = uuid.New()
	}

	return job
}

func jobTasksTestFixtures(t *testing.T) (context.Context, context.CancelFunc, *DB, *Job, job_compilers.AuthoredJob) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)

	authoredJob := createTestAuthoredJobWithTasks()
	dbJob := persistAuthoredJob(t, ctx, db, authoredJob)

	return ctx, cancel, db, dbJob, authoredJob
}

// This created Test Jobs using the new function createTestAuthoredJobWithNumTasks so that you can set the number of tasks
func jobTasksTestFixturesWithTaskNum(t *testing.T, numtasks int) (context.Context, context.CancelFunc, *DB, *Job, job_compilers.AuthoredJob) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeoutlong)

	authoredJob := createTestAuthoredJobWithNumTasks(numtasks)
	dbJob := persistAuthoredJob(t, ctx, db, authoredJob)

	return ctx, cancel, db, dbJob, authoredJob
}

func createWorker(ctx context.Context, t *testing.T, db *DB, updaters ...func(*Worker)) *Worker {
	w := Worker{
		UUID:               "f0a123a9-ab05-4ce2-8577-94802cfe74a4",
		Name:               "дрон",
		Address:            "fe80::5054:ff:fede:2ad7",
		Platform:           "linux",
		Software:           "3.0",
		Status:             api.WorkerStatusAwake,
		SupportedTaskTypes: "blender,ffmpeg,file-management",
	}

	for _, updater := range updaters {
		updater(&w)
	}

	err := db.CreateWorker(ctx, &w)
	require.NoError(t, err, "error creating worker")

	fetchedWorker, err := db.FetchWorker(ctx, w.UUID)
	require.NoError(t, err, "error fetching worker")
	require.NotNil(t, fetchedWorker, "fetched worker is nil, but no error returned")

	return fetchedWorker
}

// createWorkerFrom duplicates the given worker, ensuring new UUIDs.
func createWorkerFrom(ctx context.Context, t *testing.T, db *DB, worker Worker) *Worker {
	newWorker := sqlc.Worker{
		UUID:               uuid.New(),
		Secret:             worker.Secret,
		Name:               worker.Name + " (copy)",
		Address:            worker.Address,
		Platform:           worker.Platform,
		Software:           worker.Software,
		Status:             worker.Status,
		LastSeenAt:         nullTimeToUTC(worker.LastSeenAt),
		StatusRequested:    worker.StatusRequested,
		LazyStatusRequest:  worker.LazyStatusRequest,
		SupportedTaskTypes: worker.SupportedTaskTypes,
		DeletedAt:          worker.DeletedAt,
		CanRestart:         worker.CanRestart,
	}

	err := db.CreateWorker(ctx, &newWorker)
	require.NoError(t, err)

	dbWorker, err := db.FetchWorker(ctx, newWorker.UUID)
	require.NoError(t, err)

	return dbWorker
}

func countTaskFailures(ctx context.Context, db *DB) int {
	queries := db.queries()
	numFailures, err := queries.Test_CountTaskFailures(ctx)
	if err != nil {
		panic(err)
	}

	if numFailures > math.MaxInt {
		panic(fmt.Sprintf("too many failures: %v", numFailures))
	}
	return int(numFailures)
}
