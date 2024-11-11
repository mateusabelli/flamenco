package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projects.blender.org/studio/flamenco/internal/manager/job_compilers"
	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
	"projects.blender.org/studio/flamenco/internal/uuid"
	"projects.blender.org/studio/flamenco/pkg/api"
)

const schedulerTestTimeout = 100 * time.Millisecond
const schedulerTestTimeoutlong = 5000 * time.Millisecond

func TestNoTasks(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	w := linuxWorker(t, db)

	task, err := db.ScheduleTask(ctx, &w)
	assert.Nil(t, task)
	require.NoError(t, err)
}

func TestOneJobOneTask(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	w := linuxWorker(t, db)

	authTask := authorTestTask("the task", "blender")
	atj := authorTestJob("b6a1d859-122f-4791-8b78-b943329a9989", "simple-blender-render", authTask)
	job := constructTestJob(ctx, t, db, atj)

	task, err := db.ScheduleTask(ctx, &w)
	require.NoError(t, err)

	// Check the returned task.
	require.NotNil(t, task)
	assert.Equal(t, job.ID, task.JobID)
	require.NotNil(t, task.WorkerID, "no worker assigned to returned task")
	assert.Equal(t, w.ID, int64(*task.WorkerID), "task must be assigned to the requesting worker")

	// Check the task in the database.
	now := db.now()
	dbTask, err := db.FetchTask(context.Background(), authTask.UUID)
	require.NoError(t, err)
	require.NotNil(t, dbTask)
	require.NotNil(t, dbTask.WorkerID, "no worker assigned to task in database")

	assert.Equal(t, w.ID, int64(*dbTask.WorkerID), "task must be assigned to the requesting worker")
	assert.WithinDuration(t, now, dbTask.LastTouchedAt, time.Second, "task must be 'touched' by the worker after scheduling")
}

func TestOneJobThreeTasksByPrio(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	w := linuxWorker(t, db)

	att1 := authorTestTask("1 low-prio task", "blender")
	att2 := authorTestTask("2 high-prio task", "ffmpeg")
	att2.Priority = 100
	att3 := authorTestTask("3 low-prio task", "blender")
	atj := authorTestJob(
		"1295757b-e668-4c49-8b89-f73db8270e42",
		"simple-blender-render",
		att1, att2, att3)

	job := constructTestJob(ctx, t, db, atj)

	task, err := db.ScheduleTask(ctx, &w)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, job.ID, task.JobID)
	assert.NotNil(t, task.Job)

	assert.Equal(t, att2.Name, task.Name, "the high-prio task should have been chosen")
}

func TestOneJobThreeTasksByDependencies(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	w := linuxWorker(t, db)

	att1 := authorTestTask("1 low-prio task", "blender")
	att2 := authorTestTask("2 high-prio task", "ffmpeg")
	att2.Priority = 100
	att2.Dependencies = []*job_compilers.AuthoredTask{&att1}
	att3 := authorTestTask("3 low-prio task", "blender")
	atj := authorTestJob(
		"1295757b-e668-4c49-8b89-f73db8270e42",
		"simple-blender-render",
		att1, att2, att3)
	job := constructTestJob(ctx, t, db, atj)

	task, err := db.ScheduleTask(ctx, &w)
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, job.ID, task.JobID)
	assert.Equal(t, att1.Name, task.Name, "the first task should have been chosen")
}

func TestTwoJobsThreeTasks(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	w := linuxWorker(t, db)

	att1_1 := authorTestTask("1.1 low-prio task", "blender")
	att1_2 := authorTestTask("1.2 high-prio task", "ffmpeg")
	att1_2.Priority = 100
	att1_2.Dependencies = []*job_compilers.AuthoredTask{&att1_1}
	att1_3 := authorTestTask("1.3 low-prio task", "blender")
	atj1 := authorTestJob(
		"1295757b-e668-4c49-8b89-f73db8270e42",
		"simple-blender-render",
		att1_1, att1_2, att1_3)

	att2_1 := authorTestTask("2.1 low-prio task", "blender")
	att2_2 := authorTestTask("2.2 high-prio task", "ffmpeg")
	att2_2.Priority = 100
	att2_2.Dependencies = []*job_compilers.AuthoredTask{&att2_1}
	att2_3 := authorTestTask("2.3 highest-prio task", "blender")
	att2_3.Priority = 150
	atj2 := authorTestJob(
		"7180617b-da70-411c-8b38-b972ab2bae8d",
		"simple-blender-render",
		att2_1, att2_2, att2_3)
	atj2.Priority = 100 // Increase priority over job 1.

	constructTestJob(ctx, t, db, atj1)
	job2 := constructTestJob(ctx, t, db, atj2)

	task, err := db.ScheduleTask(ctx, &w)
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, job2.ID, task.JobID)
	assert.Equal(t, att2_3.Name, task.Name, "the 3rd task of the 2nd job should have been chosen")
}

// TestFanOutFanIn tests one starting task, then multiple tasks that depend on
// it that can run in parallel (fan-out), then one task that depends on all the
// parallel tasks (fan-in), and finally one last task that depends on the fan-in
// task.
func TestFanOutFanIn(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	w := linuxWorker(t, db)

	// Single start task.
	task1 := authorTestTask("1 start", "blender")

	// Fan out.
	task2_1 := authorTestTask("2.1 parallel", "blender")
	task2_1.Dependencies = []*job_compilers.AuthoredTask{&task1}
	task2_2 := authorTestTask("2.2 parallel", "blender")
	task2_2.Dependencies = []*job_compilers.AuthoredTask{&task1}
	task2_3 := authorTestTask("2.3 parallel", "blender")
	task2_3.Dependencies = []*job_compilers.AuthoredTask{&task1}

	// Fan in.
	task3 := authorTestTask("3 fan-in", "blender")
	task3.Dependencies = []*job_compilers.AuthoredTask{&task2_1, &task2_2, &task2_3}

	// Final task.
	task4 := authorTestTask("4 final", "ffmpeg")
	task4.Dependencies = []*job_compilers.AuthoredTask{&task3}

	// Construct the job, with the tasks not in execution order, to root out
	// potential issues with the dependency resolution.
	atj := authorTestJob(
		"92e75ecf-7d2a-461c-8443-2fbe6a8b559d",
		"fan-out-fan-in",
		task4, task3, task2_1, task2_2, task1, task2_3)
	require.NotNil(t, constructTestJob(ctx, t, db, atj))

	// Check the order in which tasks are handed out.
	executionOrder := []string{} // Slice of task names.
	for index := range 6 {
		task, err := db.ScheduleTask(ctx, &w)
		require.NoError(t, err)
		require.NotNil(t, task, "task #%d is nil", index)
		executionOrder = append(executionOrder, task.Name)

		// Fake that the task has been completed by the worker.
		task.Status = api.TaskStatusCompleted
		require.NoError(t, db.SaveTaskStatus(ctx, task))
	}

	expectedOrder := []string{
		"1 start",
		"2.1 parallel",
		"2.2 parallel",
		"2.3 parallel",
		"3 fan-in",
		"4 final",
	}
	assert.Equal(t, expectedOrder, executionOrder)
}

func TestSomeButNotAllDependenciesCompleted(t *testing.T) {
	// There was a bug in the task scheduler query, where it would schedule a task
	// if any of its dependencies was completed (instead of all dependencies).
	// This test reproduces that problematic scenario.
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	att1 := authorTestTask("1.1 completed task", "blender")
	att2 := authorTestTask("1.2 queued task of unsupported type", "unsupported")
	att3 := authorTestTask("1.3 queued task with queued dependency", "ffmpeg")
	att3.Dependencies = []*job_compilers.AuthoredTask{&att1, &att2}

	atj := authorTestJob("1295757b-e668-4c49-8b89-f73db8270e42", "simple-blender-render", att1, att2, att3)
	constructTestJob(ctx, t, db, atj)

	// Complete the first task. The other two are still `queued`.
	setTaskStatus(t, db, att1.UUID, api.TaskStatusCompleted)

	w := linuxWorker(t, db)
	task, err := db.ScheduleTask(ctx, &w)
	require.NoError(t, err)
	if task != nil {
		t.Fatalf("there should not be any task assigned, but received %q", task.Name)
	}
}

func TestAlreadyAssigned(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	w := linuxWorker(t, db)

	att1 := authorTestTask("1 low-prio task", "blender")
	att2 := authorTestTask("2 high-prio task", "ffmpeg")
	att2.Priority = 100
	att3 := authorTestTask("3 low-prio task", "blender")
	atj := authorTestJob(
		"1295757b-e668-4c49-8b89-f73db8270e42",
		"simple-blender-render",
		att1, att2, att3)

	constructTestJob(ctx, t, db, atj)

	// Assign the task to the worker, and mark it as Active.
	// This should make it get returned by the scheduler, even when there is
	// another, higher-prio task to be done.
	dbTask3, err := db.FetchTask(ctx, att3.UUID)
	require.NoError(t, err)
	dbTask3.WorkerID = ptr(uint(w.ID))
	dbTask3.Status = api.TaskStatusActive
	err = db.SaveTask(ctx, dbTask3)
	require.NoError(t, err)

	task, err := db.ScheduleTask(ctx, &w)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, att3.Name, task.Name, "the already-assigned task should have been chosen")
}

func TestAssignedToOtherWorker(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	w := linuxWorker(t, db)
	w2 := windowsWorker(t, db)

	att1 := authorTestTask("1 low-prio task", "blender")
	att2 := authorTestTask("2 high-prio task", "ffmpeg")
	att2.Priority = 100
	atj := authorTestJob(
		"1295757b-e668-4c49-8b89-f73db8270e42",
		"simple-blender-render",
		att1, att2)

	constructTestJob(ctx, t, db, atj)

	// Assign the high-prio task to the other worker. Because the task is queued,
	// it shouldn't matter which worker it's assigned to.
	dbTask2, err := db.FetchTask(ctx, att2.UUID)
	require.NoError(t, err)
	dbTask2.WorkerID = ptr(uint(w2.ID))
	dbTask2.Status = api.TaskStatusQueued
	err = db.SaveTask(ctx, dbTask2)
	require.NoError(t, err)

	task, err := db.ScheduleTask(ctx, &w)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, att2.Name, task.Name, "the high-prio task should have been chosen")
	assert.Equal(t, int64(*task.WorkerID), w.ID, "the task should now be assigned to the worker it was scheduled for")
}

func TestPreviouslyFailed(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	w := linuxWorker(t, db)

	att1 := authorTestTask("1 failed task", "blender")
	att2 := authorTestTask("2 expected task", "blender")
	atj := authorTestJob(
		"1295757b-e668-4c49-8b89-f73db8270e42",
		"simple-blender-render",
		att1, att2)
	job := constructTestJob(ctx, t, db, atj)

	// Mimick that this worker already failed the first task.
	tasks, err := db.FetchTasksOfJob(ctx, job)
	require.NoError(t, err)
	numFailed, err := db.AddWorkerToTaskFailedList(ctx, tasks[0], &w)
	require.NoError(t, err)
	assert.Equal(t, 1, numFailed)

	// This should assign the 2nd task.
	task, err := db.ScheduleTask(ctx, &w)
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, att2.Name, task.Name, "the second task should have been chosen")
}

func TestWorkerTagJobWithTag(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	// Create worker tags:
	tag1 := WorkerTag{UUID: "f0157623-4b14-4801-bee2-271dddab6309", Name: "Tag 1"}
	tag2 := WorkerTag{UUID: "2f71dba1-cf92-4752-8386-f5926affabd5", Name: "Tag 2"}
	require.NoError(t, db.CreateWorkerTag(ctx, &tag1))
	require.NoError(t, db.CreateWorkerTag(ctx, &tag2))

	// Create a worker in tag1:
	workerC := linuxWorker(t, db)
	require.NoError(t, db.WorkerSetTags(ctx, &workerC, []string{tag1.UUID}))

	// Create a worker without tag:
	workerNC := linuxWorker(t, db, func(w *Worker) {
		w.UUID = "c53f8f68-4149-4790-991c-ba73a326551e"
	})

	{ // Test job with different tag:
		authTask := authorTestTask("the task", "blender")
		job := authorTestJob("499cf0f8-e83d-4cb1-837a-df94789d07db", "simple-blender-render", authTask)
		job.WorkerTagUUID = tag2.UUID
		constructTestJob(ctx, t, db, job)

		task, err := db.ScheduleTask(ctx, &workerC)
		require.NoError(t, err)
		assert.Nil(t, task, "job with different tag should not be scheduled")
	}

	{ // Test job with matching tag:
		authTask := authorTestTask("the task", "blender")
		job := authorTestJob("5d4c2321-0bb7-4c13-a9dd-32a2c0cd156e", "simple-blender-render", authTask)
		job.WorkerTagUUID = tag1.UUID
		constructTestJob(ctx, t, db, job)

		task, err := db.ScheduleTask(ctx, &workerC)
		require.NoError(t, err)
		require.NotNil(t, task, "job with matching tag should be scheduled")
		assert.Equal(t, authTask.UUID, task.UUID)

		task, err = db.ScheduleTask(ctx, &workerNC)
		require.NoError(t, err)
		assert.Nil(t, task, "job with tag should not be scheduled for worker without tag")
	}
}

func TestWorkerTagJobWithoutTag(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	// Create worker tag:
	tag1 := WorkerTag{UUID: "f0157623-4b14-4801-bee2-271dddab6309", Name: "Tag 1"}
	require.NoError(t, db.CreateWorkerTag(ctx, &tag1))

	// Create a worker in tag1:
	workerC := linuxWorker(t, db)
	require.NoError(t, db.WorkerSetTags(ctx, &workerC, []string{tag1.UUID}))

	// Create a worker without tag:
	workerNC := linuxWorker(t, db, func(w *Worker) {
		w.UUID = "c53f8f68-4149-4790-991c-ba73a326551e"
	})

	// Test tag-less job:
	authTask := authorTestTask("the task", "blender")
	job := authorTestJob("b6a1d859-122f-4791-8b78-b943329a9989", "simple-blender-render", authTask)
	constructTestJob(ctx, t, db, job)

	task, err := db.ScheduleTask(ctx, &workerC)
	require.NoError(t, err)
	require.NotNil(t, task, "job without tag should always be scheduled to worker in some tag")
	assert.Equal(t, authTask.UUID, task.UUID)

	task, err = db.ScheduleTask(ctx, &workerNC)
	require.NoError(t, err)
	require.NotNil(t, task, "job without tag should always be scheduled to worker without tag")
	assert.Equal(t, authTask.UUID, task.UUID)
}

func TestBlocklisted(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	w := linuxWorker(t, db)

	att1 := authorTestTask("1 blocked task", "blender")
	att2 := authorTestTask("2 expected task", "ffmpeg")
	atj := authorTestJob(
		"1295757b-e668-4c49-8b89-f73db8270e42",
		"simple-blender-render",
		att1, att2)
	job := constructTestJob(ctx, t, db, atj)

	// Mimick that this worker was already blocked for 'blender' tasks of this job.
	err := db.AddWorkerToJobBlocklist(ctx, job, &w, "blender")
	require.NoError(t, err)

	// This should assign the 2nd task.
	task, err := db.ScheduleTask(ctx, &w)
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, att2.Name, task.Name, "the second task should have been chosen")
}

// To test: blocklists

// To test: variable replacement

func constructTestJob(
	ctx context.Context, t *testing.T, db *DB, authoredJob job_compilers.AuthoredJob,
) *Job {
	err := db.StoreAuthoredJob(ctx, authoredJob)
	require.NoError(t, err, "storing authored job")

	dbJob, err := db.FetchJob(ctx, authoredJob.JobID)
	require.NoError(t, err, "fetching authored job")

	// Queue the job.
	dbJob.Status = api.JobStatusQueued
	err = db.SaveJobStatus(ctx, dbJob)
	require.NoError(t, err, "queueing job")

	return dbJob
}

func authorTestJob(jobUUID, jobType string, tasks ...job_compilers.AuthoredTask) job_compilers.AuthoredJob {
	job := job_compilers.AuthoredJob{
		JobID:    jobUUID,
		Name:     "test job",
		JobType:  jobType,
		Priority: 50,
		Status:   api.JobStatusQueued,
		Created:  time.Now(),
		Tasks:    tasks,
	}
	return job
}

func authorTestTask(name, taskType string, dependencies ...*job_compilers.AuthoredTask) job_compilers.AuthoredTask {
	task := job_compilers.AuthoredTask{
		Name:         name,
		Type:         taskType,
		UUID:         uuid.New(),
		Priority:     50,
		Commands:     make([]job_compilers.AuthoredCommand, 0),
		Dependencies: dependencies,
	}
	return task
}

func setTaskStatus(t *testing.T, db *DB, taskUUID string, status api.TaskStatus) {
	ctx := context.Background()
	task, err := db.FetchTask(ctx, taskUUID)
	require.NoError(t, err)

	task.Status = status

	require.NoError(t, db.SaveTask(ctx, task))
}

func linuxWorker(t *testing.T, db *DB, updaters ...func(worker *Worker)) Worker {
	w := Worker{
		UUID:               "b13b8322-3e96-41c3-940a-3d581008a5f8",
		Name:               "Linux",
		Platform:           "linux",
		Status:             api.WorkerStatusAwake,
		SupportedTaskTypes: "blender,ffmpeg,file-management,misc",
	}

	for _, updater := range updaters {
		updater(&w)
	}

	saveTestWorker(t, db, &w)
	return w
}

func windowsWorker(t *testing.T, db *DB) Worker {
	w := Worker{
		UUID:               "4f6ee45e-c8fc-4c31-bf5c-922f2415deb1",
		Name:               "Windows",
		Platform:           "windows",
		Status:             api.WorkerStatusAwake,
		SupportedTaskTypes: "blender,ffmpeg,file-management,misc",
	}

	saveTestWorker(t, db, &w)
	return w
}

func saveTestWorker(t *testing.T, db *DB, worker *Worker) {
	params := sqlc.CreateWorkerParams{
		CreatedAt:          db.now(),
		UUID:               worker.UUID,
		Secret:             worker.Secret,
		Name:               worker.Name,
		Address:            worker.Address,
		Platform:           worker.Platform,
		Software:           worker.Software,
		Status:             worker.Status,
		LastSeenAt:         nullTimeToUTC(worker.LastSeenAt),
		StatusRequested:    worker.StatusRequested,
		LazyStatusRequest:  worker.LazyStatusRequest,
		SupportedTaskTypes: worker.SupportedTaskTypes,
		DeletedAt:          sql.NullTime(worker.DeletedAt),
		CanRestart:         worker.CanRestart,
	}

	queries := db.queries()
	id, err := queries.CreateWorker(context.TODO(), params)
	require.NoError(t, err, "cannot save worker %q", worker.Name)

	worker.ID = id
	worker.CreatedAt = params.CreatedAt
}
