package task_state_machine

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

func TestRequeueActiveTasksOfWorker(t *testing.T) {
	mockCtrl, ctx, sm, mocks := taskStateMachineTestFixtures(t)
	defer mockCtrl.Finish()

	worker := persistence.Worker{
		UUID: "3ed470c8-d41e-4668-92d0-d799997433a4",
		Name: "testert",
	}

	// Mock that the worker has two active tasks. It shouldn't happen, but even
	// when it does, both should be requeued when the worker signs off.
	task1, job := taskWithStatus(api.JobStatusActive, api.TaskStatusActive)
	task2 := taskOfSameJob(task1, api.TaskStatusActive)
	workerTasks := []persistence.TaskJob{
		{Task: *task1, JobUUID: job.UUID},
		{Task: *task2, JobUUID: job.UUID},
	}

	task1PrevStatus := task1.Status
	task2PrevStatus := task2.Status

	mocks.persist.EXPECT().FetchJob(ctx, job.UUID).Return(job, nil).Times(len(workerTasks))
	mocks.persist.EXPECT().FetchTasksOfWorkerInStatus(ctx, &worker, api.TaskStatusActive).Return(workerTasks, nil)

	// Expect this re-queueing to end up in the task's log and activity.
	logMsg1 := "task changed status active -> queued"
	logMsg2 := "Task was queued by Manager because worker had to test"
	task1WithActivity := *task1
	task1WithActivity.Activity = logMsg2
	task2WithActivity := *task2
	task2WithActivity.Activity = logMsg2
	task1WithActivityAndStatus := task1WithActivity
	task1WithActivityAndStatus.Status = api.TaskStatusQueued
	task2WithActivityAndStatus := task2WithActivity
	task2WithActivityAndStatus.Status = api.TaskStatusQueued
	mocks.persist.EXPECT().SaveTaskActivity(ctx, &task1WithActivity)
	mocks.persist.EXPECT().SaveTaskActivity(ctx, &task2WithActivity)
	mocks.persist.EXPECT().SaveTaskStatus(ctx, &task1WithActivityAndStatus)
	mocks.persist.EXPECT().SaveTaskStatus(ctx, &task2WithActivityAndStatus)

	mocks.logStorage.EXPECT().WriteTimestamped(gomock.Any(), job.UUID, task1.UUID, logMsg1)
	mocks.logStorage.EXPECT().WriteTimestamped(gomock.Any(), job.UUID, task2.UUID, logMsg1)

	mocks.logStorage.EXPECT().WriteTimestamped(gomock.Any(), job.UUID, task1.UUID, logMsg2)
	mocks.logStorage.EXPECT().WriteTimestamped(gomock.Any(), job.UUID, task2.UUID, logMsg2)

	mocks.broadcaster.EXPECT().BroadcastTaskUpdate(api.EventTaskUpdate{
		Activity:       logMsg2,
		Id:             task1.UUID,
		JobId:          job.UUID,
		Name:           task1.Name,
		PreviousStatus: &task1PrevStatus,
		Status:         api.TaskStatusQueued,
		Updated:        task1.UpdatedAt.Time,
	})

	mocks.broadcaster.EXPECT().BroadcastTaskUpdate(api.EventTaskUpdate{
		Activity:       logMsg2,
		Id:             task2.UUID,
		JobId:          job.UUID,
		Name:           task2.Name,
		PreviousStatus: &task2PrevStatus,
		Status:         api.TaskStatusQueued,
		Updated:        task2.UpdatedAt.Time,
	})

	mocks.expectFetchJobOfTask(task1, job)
	mocks.expectFetchJobOfTask(task2, job)

	err := sm.RequeueActiveTasksOfWorker(ctx, &worker, "worker had to test")
	require.NoError(t, err)
}

// Check that a Worker's task goes to 'paused' when the worker signs off and the
// job is in 'pause-requested' state. This should also ensure that, if this
// happens to be the last-active task of the job, that the entire job goes to
// `paused` state.
func TestPauseActiveTasksOfWorker(t *testing.T) {
	mockCtrl, ctx, sm, mocks := taskStateMachineTestFixtures(t)
	defer mockCtrl.Finish()

	worker := persistence.Worker{
		UUID: "3ed470c8-d41e-4668-92d0-d799997433a4",
		Name: "testert",
	}

	// Mock that the job is in pause-requested, with one paused task and one active task.
	task1, job := taskWithStatus(api.JobStatusPauseRequested, api.TaskStatusActive)
	taskOfSameJob(task1, api.TaskStatusPaused) // Create a 2nd, already-paused task.

	jobPrevStatus := job.Status
	task1PrevStatus := task1.Status

	mocks.persist.EXPECT().FetchJob(ctx, job.UUID).Return(job, nil)
	mocks.persist.EXPECT().FetchTasksOfWorkerInStatus(ctx, &worker, api.TaskStatusActive).
		Return([]persistence.TaskJob{{Task: *task1, JobUUID: job.UUID}}, nil)

	mocks.persist.EXPECT().CountTasksOfJobInStatus(ctx, job, api.TaskStatusActive).Return(0, 2, nil) // 0 of 2 active.

	// Expect this re-queueing to end up in the task's log and activity.
	logMsg1 := "task changed status active -> paused"
	logMsg2 := "Task was paused by Manager because worker had to test"
	task1WithActivity := *task1
	task1WithActivity.Activity = logMsg2
	task1WithActivityAndStatus := task1WithActivity
	task1WithActivityAndStatus.Status = api.TaskStatusPaused
	mocks.persist.EXPECT().SaveTaskActivity(ctx, &task1WithActivity)
	mocks.persist.EXPECT().SaveTaskStatus(ctx, &task1WithActivityAndStatus)

	jobWithStatus := *job
	jobWithStatus.Status = api.JobStatusPaused
	jobWithStatus.Activity = "Changed to status \"paused\": last task got paused"
	mocks.persist.EXPECT().SaveJobStatus(ctx, &jobWithStatus)

	mocks.logStorage.EXPECT().WriteTimestamped(gomock.Any(), job.UUID, task1.UUID, logMsg1)
	mocks.logStorage.EXPECT().WriteTimestamped(gomock.Any(), job.UUID, task1.UUID, logMsg2)

	mocks.broadcaster.EXPECT().BroadcastTaskUpdate(api.EventTaskUpdate{
		Activity:       logMsg2,
		Id:             task1.UUID,
		JobId:          job.UUID,
		Name:           task1.Name,
		PreviousStatus: &task1PrevStatus,
		Status:         api.TaskStatusPaused,
		Updated:        task1.UpdatedAt.Time,
	})
	mocks.broadcaster.EXPECT().BroadcastJobUpdate(api.EventJobUpdate{
		Id:             job.UUID,
		Name:           &job.Name,
		PreviousStatus: &jobPrevStatus,
		Priority:       int(job.Priority),
		RefreshTasks:   false,
		Status:         api.JobStatusPaused,
		Type:           job.JobType,
		Updated:        job.UpdatedAt.Time,
	})

	mocks.expectFetchJobOfTask(task1, job)

	err := sm.RequeueActiveTasksOfWorker(ctx, &worker, "worker had to test")
	require.NoError(t, err)
}
