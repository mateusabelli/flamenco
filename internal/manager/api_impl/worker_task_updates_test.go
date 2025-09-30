package api_impl

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projects.blender.org/studio/flamenco/internal/manager/config"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

func TestTaskUpdate(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)
	worker := testWorker()

	// Construct the JSON request object.
	taskUpdate := api.TaskUpdateJSONRequestBody{
		Activity:   ptr("testing"),
		Log:        ptr("line1\nline2\n"),
		TaskStatus: ptr(api.TaskStatusCompleted),
	}

	// Construct the task that's supposed to be updated.
	taskID := "181eab68-1123-4790-93b1-94309a899411"
	jobID := "e4719398-7cfa-4877-9bab-97c2d6c158b5"
	mockJob := persistence.Job{ID: 1234, UUID: jobID}
	mockTask := persistence.Task{
		UUID:     taskID,
		WorkerID: sql.NullInt64{Int64: worker.ID, Valid: true},
		JobID:    mockJob.ID,
		Activity: "pre-update activity",
	}

	// Expect the task to be fetched.
	taskJobWorker := persistence.TaskJobWorker{
		Task:       mockTask,
		JobUUID:    jobID,
		WorkerUUID: worker.UUID,
	}
	mf.persistence.EXPECT().FetchTask(gomock.Any(), taskID).Return(taskJobWorker, nil)

	// Expect the task status change to be handed to the state machine.
	var statusChangedtask persistence.Task
	mf.stateMachine.EXPECT().TaskStatusChange(gomock.Any(), gomock.AssignableToTypeOf(&persistence.Task{}), api.TaskStatusCompleted).
		DoAndReturn(func(ctx context.Context, task *persistence.Task, newStatus api.TaskStatus) error {
			statusChangedtask = *task
			return nil
		})

	// Expect the activity to be updated.
	var actUpdatedTask persistence.Task
	mf.persistence.EXPECT().SaveTaskActivity(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, task *persistence.Task) error {
			actUpdatedTask = *task
			return nil
		})

	// Expect the log to be written and broadcast over SocketIO.
	mf.logStorage.EXPECT().Write(gomock.Any(), jobID, taskID, "line1\nline2\n")

	// Expect a 'touch' of the task.
	var touchedTaskUUID string
	mf.persistence.EXPECT().TaskTouchedByWorker(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, taskUUID string) error {
			touchedTaskUUID = taskUUID
			return nil
		})
	mf.persistence.EXPECT().WorkerSeen(gomock.Any(), &worker)

	// Do the call.
	echoCtx := mf.prepareMockedJSONRequest(taskUpdate)
	requestWorkerStore(echoCtx, &worker)
	err := mf.flamenco.TaskUpdate(echoCtx, taskID)

	// Check the saved task.
	require.NoError(t, err)
	assert.Equal(t, mockTask.UUID, statusChangedtask.UUID)
	assert.Equal(t, mockTask.UUID, actUpdatedTask.UUID)
	assert.Equal(t, mockTask.UUID, touchedTaskUUID)
	assert.Equal(t, "testing", statusChangedtask.Activity)
	assert.Equal(t, "testing", actUpdatedTask.Activity)
}

func TestTaskUpdateStepsCompleted(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	worker := testWorker()
	assignedWorker := api.AssignedWorker{
		Name: worker.Name,
		Uuid: worker.UUID,
	}

	// Construct the JSON request object.
	taskUpdate := api.TaskUpdateJSONRequestBody{
		Activity:       ptr("testing"),
		Log:            ptr("line1\nline2\n"),
		StepsCompleted: ptr(2),
	}

	// Construct the task that's supposed to be updated.
	taskID := "181eab68-1123-4790-93b1-94309a899411"
	jobID := "e4719398-7cfa-4877-9bab-97c2d6c158b5"
	mockJob := persistence.Job{
		ID:             1234,
		UUID:           jobID,
		Name:           "mock jobs",
		StepsCompleted: 0,
		StepsTotal:     3,
	}
	mockTask := persistence.Task{
		ID:         47,
		UUID:       taskID,
		WorkerID:   sql.NullInt64{Int64: worker.ID, Valid: true},
		JobID:      mockJob.ID,
		Activity:   "pre-update activity",
		StepsTotal: 3,
	}

	// Expect the task to be fetched.
	taskJobWorker := persistence.TaskJobWorker{
		Task:       mockTask,
		JobUUID:    jobID,
		WorkerUUID: worker.UUID,
	}
	mf.persistence.EXPECT().FetchTask(gomock.Any(), taskID).
		Return(taskJobWorker, nil).
		Times(2)

	// Expect the activity to be updated.
	var actUpdatedTask persistence.Task
	mf.persistence.EXPECT().SaveTaskActivity(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, task *persistence.Task) error {
			actUpdatedTask = *task
			return nil
		}).
		Times(2)

	// Expect the log to be written and broadcast over SocketIO.
	mf.logStorage.EXPECT().Write(gomock.Any(), jobID, taskID, "line1\nline2\n").Times(2)

	// Expect the step count to be updated and broadcast.
	mf.persistence.EXPECT().SaveTaskStepsCompleted(gomock.Any(), mockJob.ID, mockTask.ID, int64(2))
	mf.broadcaster.EXPECT().BroadcastTaskUpdate(api.EventTaskUpdate{
		Activity:       "testing",
		Id:             mockTask.UUID,
		JobId:          jobID,
		Name:           mockTask.Name,
		Status:         mockTask.Status,
		StepsCompleted: 2,
		StepsTotal:     3,
		Worker:         &assignedWorker,
	})

	mockJobWith2CompletedSteps := mockJob
	mockJobWith2CompletedSteps.StepsCompleted = 2
	mf.persistence.EXPECT().FetchJobByID(gomock.Any(), mockJob.ID).
		Return(&mockJobWith2CompletedSteps, nil)
	mf.broadcaster.EXPECT().BroadcastJobUpdate(api.EventJobUpdate{
		Id:             mockJob.UUID,
		Name:           &mockJob.Name,
		Status:         mockJob.Status,
		StepsCompleted: 2,
		StepsTotal:     3,
	})

	// Expect a 'touch' of the task.
	mf.persistence.EXPECT().TaskTouchedByWorker(gomock.Any(), taskID).Times(2)
	mf.persistence.EXPECT().WorkerSeen(gomock.Any(), &worker).Times(2)

	// Do the call.
	echoCtx := mf.prepareMockedJSONRequest(taskUpdate)
	requestWorkerStore(echoCtx, &worker)
	err := mf.flamenco.TaskUpdate(echoCtx, taskID)

	// Check the saved task.
	require.NoError(t, err)
	assert.Equal(t, mockTask.UUID, actUpdatedTask.UUID)
	assert.Equal(t, "testing", actUpdatedTask.Activity)

	// Do another update, the step count should be clamped to the task's total step count.
	taskUpdate.StepsCompleted = ptr(47)
	mf.persistence.EXPECT().SaveTaskStepsCompleted(gomock.Any(), mockJob.ID, mockTask.ID, int64(3))
	mf.broadcaster.EXPECT().BroadcastTaskUpdate(api.EventTaskUpdate{
		Activity:       "testing",
		Id:             mockTask.UUID,
		JobId:          jobID,
		Name:           mockTask.Name,
		Status:         mockTask.Status,
		StepsCompleted: 3,
		StepsTotal:     3,
		Worker:         &assignedWorker,
	})

	mockJobWith3CompletedSteps := mockJob
	mockJobWith3CompletedSteps.StepsCompleted = 3
	mf.persistence.EXPECT().FetchJobByID(gomock.Any(), mockJob.ID).
		Return(&mockJobWith3CompletedSteps, nil)
	mf.broadcaster.EXPECT().BroadcastJobUpdate(api.EventJobUpdate{
		Id:             mockJob.UUID,
		Name:           &mockJob.Name,
		Status:         mockJob.Status,
		StepsCompleted: 3,
		StepsTotal:     3,
	})

	// Do the call.
	echoCtx = mf.prepareMockedJSONRequest(taskUpdate)
	requestWorkerStore(echoCtx, &worker)
	err = mf.flamenco.TaskUpdate(echoCtx, taskID)
	require.NoError(t, err)
}

func TestTaskUpdateFailed(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)
	worker := testWorker()

	// Construct the JSON request object.
	taskUpdate := api.TaskUpdateJSONRequestBody{
		TaskStatus: ptr(api.TaskStatusFailed),
	}

	// Construct the task that's supposed to be updated.
	taskID := "181eab68-1123-4790-93b1-94309a899411"
	jobID := "e4719398-7cfa-4877-9bab-97c2d6c158b5"
	mockJob := persistence.Job{ID: 1234, UUID: jobID}
	mockTask := persistence.Task{
		UUID:     taskID,
		WorkerID: sql.NullInt64{Int64: worker.ID, Valid: true},
		JobID:    mockJob.ID,
		Activity: "pre-update activity",
		Type:     "misc",
	}

	conf := config.Conf{
		Base: config.Base{
			TaskFailAfterSoftFailCount: 3,
			BlocklistThreshold:         65535, // This test doesn't cover blocklisting.
		},
	}
	mf.config.EXPECT().Get().Return(&conf).AnyTimes()

	const numSubTests = 2
	// Expect the task to be fetched for each sub-test:
	taskJobWorker := persistence.TaskJobWorker{
		Task:       mockTask,
		JobUUID:    jobID,
		WorkerUUID: worker.UUID,
	}
	mf.persistence.EXPECT().FetchTask(gomock.Any(), taskID).Return(taskJobWorker, nil).Times(numSubTests)

	// Expect a 'touch' of the task for each sub-test:
	mf.persistence.EXPECT().TaskTouchedByWorker(gomock.Any(), taskID).Times(numSubTests)
	mf.persistence.EXPECT().WorkerSeen(gomock.Any(), &worker).Times(numSubTests)

	// Mimick that this is always first failure of this worker/job/tasktype combo:
	mf.persistence.EXPECT().CountTaskFailuresOfWorker(gomock.Any(), jobID, worker.ID, "misc").Return(0, nil).Times(numSubTests)

	{
		// Expect the Worker to be added to the list of failed workers.
		// This returns 1, which is less than the failure threshold -> soft failure expected.
		mf.persistence.EXPECT().AddWorkerToTaskFailedList(gomock.Any(), &mockTask, &worker).Return(1, nil)

		mf.persistence.EXPECT().FetchJobByID(gomock.Any(), mockTask.JobID).Return(&mockJob, nil)
		mf.persistence.EXPECT().WorkersLeftToRun(gomock.Any(), &mockJob, "misc").
			Return(map[string]bool{"60453eec-5a26-43e9-9da2-d00506d492cc": true, "ce312357-29cd-4389-81ab-4d43e30945f8": true}, nil)
		mf.persistence.EXPECT().FetchTaskFailureList(gomock.Any(), &mockTask).
			Return([]*persistence.Worker{ /* It shouldn't matter whether the failing worker is here or not. */ }, nil)

		// Expect soft failure.
		mf.stateMachine.EXPECT().TaskStatusChange(gomock.Any(), &mockTask, api.TaskStatusSoftFailed)
		mf.logStorage.EXPECT().WriteTimestamped(gomock.Any(), jobID, taskID,
			"Task failed by 1 worker, Manager will mark it as soft failure. 2 more failures will cause hard failure.")

		// Do the call.
		echoCtx := mf.prepareMockedJSONRequest(taskUpdate)
		requestWorkerStore(echoCtx, &worker)
		err := mf.flamenco.TaskUpdate(echoCtx, taskID)
		require.NoError(t, err)
		assertResponseNoContent(t, echoCtx)
	}

	{
		// Test with more (mocked) failures in the past, pushing the task over the threshold.
		mf.persistence.EXPECT().AddWorkerToTaskFailedList(gomock.Any(), &mockTask, &worker).
			Return(conf.TaskFailAfterSoftFailCount, nil)
		mf.stateMachine.EXPECT().TaskStatusChange(gomock.Any(), &mockTask, api.TaskStatusFailed)
		mf.logStorage.EXPECT().WriteTimestamped(gomock.Any(), jobID, taskID,
			"Task failed by 3 workers, Manager will mark it as hard failure")

		// Do the call.
		echoCtx := mf.prepareMockedJSONRequest(taskUpdate)
		requestWorkerStore(echoCtx, &worker)
		err := mf.flamenco.TaskUpdate(echoCtx, taskID)
		require.NoError(t, err)
		assertResponseNoContent(t, echoCtx)
	}
}

func TestBlockingAfterFailure(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)
	worker := testWorker()

	// Construct the JSON request object.
	taskUpdate := api.TaskUpdateJSONRequestBody{
		TaskStatus: ptr(api.TaskStatusFailed),
	}

	// Construct the task that's supposed to be updated.
	taskID := "181eab68-1123-4790-93b1-94309a899411"
	jobID := "e4719398-7cfa-4877-9bab-97c2d6c158b5"
	mockJob := persistence.Job{ID: 1234, UUID: jobID}
	mockTask := persistence.Task{
		UUID:     taskID,
		WorkerID: sql.NullInt64{Int64: worker.ID, Valid: true},
		JobID:    mockJob.ID,
		Activity: "pre-update activity",
		Type:     "misc",
	}

	conf := config.Conf{
		Base: config.Base{
			TaskFailAfterSoftFailCount: 3,
			BlocklistThreshold:         3,
		},
	}
	mf.config.EXPECT().Get().Return(&conf).AnyTimes()

	const numSubTests = 3
	// Expect the task to be fetched for each sub-test:
	taskJobWorker := persistence.TaskJobWorker{
		Task:       mockTask,
		JobUUID:    jobID,
		WorkerUUID: worker.UUID,
	}
	mf.persistence.EXPECT().FetchTask(gomock.Any(), taskID).Return(taskJobWorker, nil).Times(numSubTests)

	// Expect a 'touch' of the task for each sub-test:
	mf.persistence.EXPECT().TaskTouchedByWorker(gomock.Any(), taskID).Times(numSubTests)
	mf.persistence.EXPECT().WorkerSeen(gomock.Any(), &worker).Times(numSubTests)

	// Mimick that this is the 3rd of this worker/job/tasktype combo, and thus should trigger a block.
	// Returns 2 because there have been 2 previous failures.
	mf.persistence.EXPECT().
		CountTaskFailuresOfWorker(gomock.Any(), jobID, worker.ID, "misc").
		Return(2, nil).
		Times(numSubTests)

	// Expect the worker to be blocked.
	mf.persistence.EXPECT().
		AddWorkerToJobBlocklist(gomock.Any(), mockJob.ID, worker.ID, "misc").
		Times(numSubTests)

	{
		// Mimick that there is another worker to work on this task, so the job should continue happily.
		mf.persistence.EXPECT().FetchJobByID(gomock.Any(), mockTask.JobID).Return(&mockJob, nil).Times(2)
		mf.persistence.EXPECT().WorkersLeftToRun(gomock.Any(), &mockJob, "misc").
			Return(map[string]bool{"60453eec-5a26-43e9-9da2-d00506d492cc": true, "ce312357-29cd-4389-81ab-4d43e30945f8": true}, nil).Times(2)
		mf.persistence.EXPECT().FetchTaskFailureList(gomock.Any(), &mockTask).
			Return([]*persistence.Worker{ /* It shouldn't matter whether the failing worker is here or not. */ }, nil).Times(2)

		// Expect the Worker to be added to the list of failed workers for this task.
		// This returns 1, which is less than the failure threshold -> soft failure.
		mf.persistence.EXPECT().AddWorkerToTaskFailedList(gomock.Any(), &mockTask, &worker).Return(1, nil)

		// Expect soft failure of the task.
		mf.stateMachine.EXPECT().TaskStatusChange(gomock.Any(), &mockTask, api.TaskStatusSoftFailed)
		mf.logStorage.EXPECT().WriteTimestamped(gomock.Any(), jobID, taskID,
			"Task failed by 1 worker, Manager will mark it as soft failure. 2 more failures will cause hard failure.")

		// Because the job didn't fail in its entirety, the tasks previously failed
		// by the Worker should be requeued so they can be picked up by another.
		mf.stateMachine.EXPECT().RequeueFailedTasksOfWorkerOfJob(
			gomock.Any(), &worker, jobID,
			"worker дрон was blocked from tasks of type \"misc\"")

		// Do the call.
		echoCtx := mf.prepareMockedJSONRequest(taskUpdate)
		requestWorkerStore(echoCtx, &worker)
		err := mf.flamenco.TaskUpdate(echoCtx, taskID)
		require.NoError(t, err)
		assertResponseNoContent(t, echoCtx)
	}

	{
		// Test without any workers left to run these tasks on this job due to blocklisting. This should fail the entire job.
		mf.persistence.EXPECT().FetchJobByID(gomock.Any(), mockTask.JobID).Return(&mockJob, nil)
		mf.persistence.EXPECT().WorkersLeftToRun(gomock.Any(), &mockJob, "misc").
			Return(map[string]bool{}, nil)
		mf.persistence.EXPECT().FetchTaskFailureList(gomock.Any(), &mockTask).
			Return([]*persistence.Worker{ /* It shouldn't matter whether the failing worker is here or not. */ }, nil)

		// Expect the Worker to be added to the list of failed workers for this task.
		// This returns 1, which is less than the failure threshold -> soft failure if it were only based on this metric.
		mf.persistence.EXPECT().AddWorkerToTaskFailedList(gomock.Any(), &mockTask, &worker).Return(1, nil)

		// Expect hard failure of the task, because there are no workers left to perfom it.
		mf.stateMachine.EXPECT().TaskStatusChange(gomock.Any(), &mockTask, api.TaskStatusFailed)
		mf.logStorage.EXPECT().WriteTimestamped(gomock.Any(), jobID, taskID,
			"Task failed by worker дрон (e7632d62-c3b8-4af0-9e78-01752928952c), Manager will fail the entire job "+
				"as there are no more workers left for tasks of type \"misc\".")

		// Expect failure of the job.
		mf.stateMachine.EXPECT().
			JobStatusChange(gomock.Any(), jobID, api.JobStatusFailed, "no more workers left to run tasks of type \"misc\"")

		// Because the job failed, there is no need to re-queue any tasks previously failed by this worker.

		// Do the call.
		echoCtx := mf.prepareMockedJSONRequest(taskUpdate)
		requestWorkerStore(echoCtx, &worker)
		err := mf.flamenco.TaskUpdate(echoCtx, taskID)
		require.NoError(t, err)
		assertResponseNoContent(t, echoCtx)
	}

	{
		// Test that no worker has been blocklisted, but the one available one did fail this task.
		// This also makes the task impossible to run, and should just fail the entire job.
		theOtherFailingWorker := persistence.Worker{
			UUID: "ce312357-29cd-4389-81ab-4d43e30945f8",
		}
		mf.persistence.EXPECT().FetchJobByID(gomock.Any(), mockTask.JobID).Return(&mockJob, nil)
		mf.persistence.EXPECT().WorkersLeftToRun(gomock.Any(), &mockJob, "misc").
			Return(map[string]bool{theOtherFailingWorker.UUID: true}, nil)
		mf.persistence.EXPECT().FetchTaskFailureList(gomock.Any(), &mockTask).
			Return([]*persistence.Worker{&theOtherFailingWorker}, nil)

		// Expect the Worker to be added to the list of failed workers for this task.
		// This returns 1, which is less than the failure threshold -> soft failure if it were only based on this metric.
		mf.persistence.EXPECT().AddWorkerToTaskFailedList(gomock.Any(), &mockTask, &worker).Return(1, nil)

		// Expect hard failure of the task, because there are no workers left to perfom it.
		mf.stateMachine.EXPECT().TaskStatusChange(gomock.Any(), &mockTask, api.TaskStatusFailed)
		mf.logStorage.EXPECT().WriteTimestamped(gomock.Any(), jobID, taskID,
			"Task failed by worker дрон (e7632d62-c3b8-4af0-9e78-01752928952c), Manager will fail the entire job "+
				"as there are no more workers left for tasks of type \"misc\".")

		// Expect failure of the job.
		mf.stateMachine.EXPECT().
			JobStatusChange(gomock.Any(), jobID, api.JobStatusFailed, "no more workers left to run tasks of type \"misc\"")

		// Because the job failed, there is no need to re-queue any tasks previously failed by this worker.

		// Do the call.
		echoCtx := mf.prepareMockedJSONRequest(taskUpdate)
		requestWorkerStore(echoCtx, &worker)
		err := mf.flamenco.TaskUpdate(echoCtx, taskID)
		require.NoError(t, err)
		assertResponseNoContent(t, echoCtx)
	}
}

func TestJobFailureAfterWorkerTaskFailure(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)
	worker := testWorker()

	// Contruct the JSON request object
	taskUpdate := api.TaskUpdateJSONRequestBody{
		TaskStatus: ptr(api.TaskStatusFailed),
	}

	// Construct the task that's supposed to be updated.
	taskID := "181eab68-1123-4790-93b1-94309a899411"
	jobID := "e4719398-7cfa-4877-9bab-97c2d6c158b5"
	mockJob := persistence.Job{ID: 1234, UUID: jobID}
	mockTask := persistence.Task{
		UUID:     taskID,
		WorkerID: sql.NullInt64{Int64: worker.ID, Valid: true},
		JobID:    mockJob.ID,
		Activity: "pre-update activity",
		Type:     "misc",
	}

	conf := config.Conf{
		Base: config.Base{
			TaskFailAfterSoftFailCount: 3,
			BlocklistThreshold:         65535, // This test doesn't cover blocklisting.
		},
	}

	mf.config.EXPECT().Get().Return(&conf).Times(2)

	taskJobWorker := persistence.TaskJobWorker{
		Task:       mockTask,
		JobUUID:    jobID,
		WorkerUUID: worker.UUID,
	}
	mf.persistence.EXPECT().FetchTask(gomock.Any(), taskID).Return(taskJobWorker, nil)

	mf.persistence.EXPECT().TaskTouchedByWorker(gomock.Any(), taskID)
	mf.persistence.EXPECT().WorkerSeen(gomock.Any(), &worker)

	mf.persistence.EXPECT().CountTaskFailuresOfWorker(gomock.Any(), jobID, worker.ID, "misc").Return(0, nil)

	mf.persistence.EXPECT().AddWorkerToTaskFailedList(gomock.Any(), &mockTask, &worker).Return(1, nil)

	mf.persistence.EXPECT().FetchJobByID(gomock.Any(), mockTask.JobID).Return(&mockJob, nil)
	mf.persistence.EXPECT().WorkersLeftToRun(gomock.Any(), &mockJob, "misc").
		Return(map[string]bool{"e7632d62-c3b8-4af0-9e78-01752928952c": true}, nil)
	mf.persistence.EXPECT().FetchTaskFailureList(gomock.Any(), &mockTask).
		Return([]*persistence.Worker{ /* It shouldn't matter whether the failing worker is here or not. */ }, nil)

	// Expect hard failure of the task, because there are no workers left to perfom it.
	mf.stateMachine.EXPECT().TaskStatusChange(gomock.Any(), &mockTask, api.TaskStatusFailed)
	mf.logStorage.EXPECT().WriteTimestamped(gomock.Any(), jobID, taskID,
		"Task failed by worker дрон (e7632d62-c3b8-4af0-9e78-01752928952c), Manager will fail the entire job "+
			"as there are no more workers left for tasks of type \"misc\".")

	// Expect failure of the job.
	mf.stateMachine.EXPECT().
		JobStatusChange(gomock.Any(), jobID, api.JobStatusFailed, "no more workers left to run tasks of type \"misc\"")

	// Do the call
	echoCtx := mf.prepareMockedJSONRequest(taskUpdate)
	requestWorkerStore(echoCtx, &worker)
	err := mf.flamenco.TaskUpdate(echoCtx, taskID)
	require.NoError(t, err)
	assertResponseNoContent(t, echoCtx)
}
