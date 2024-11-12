package api_impl

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"projects.blender.org/studio/flamenco/internal/manager/config"
	"projects.blender.org/studio/flamenco/internal/manager/job_compilers"
	"projects.blender.org/studio/flamenco/internal/manager/last_rendered"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
	"projects.blender.org/studio/flamenco/pkg/moremock"
)

func ptr[T any](value T) *T {
	return &value
}

func TestSubmitJobWithoutSettings(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)
	worker := testWorker()

	submittedJob := api.SubmittedJob{
		Name:              "поднео посао",
		Type:              "test",
		Priority:          50,
		SubmitterPlatform: worker.Platform,
	}

	mf.expectConvertTwoWayVariables(t,
		config.VariableAudienceWorkers,
		config.VariablePlatform(worker.Platform),
		map[string]string{},
	)

	// Expect the job compiler to be called.
	authoredJob := job_compilers.AuthoredJob{
		JobID:    "afc47568-bd9d-4368-8016-e91d945db36d",
		Name:     submittedJob.Name,
		JobType:  submittedJob.Type,
		Priority: submittedJob.Priority,
		Status:   api.JobStatusUnderConstruction,
		Created:  mf.clock.Now(),
	}
	mf.jobCompiler.EXPECT().Compile(gomock.Any(), submittedJob).Return(&authoredJob, nil)

	// Expect the job to be saved with 'queued' status:
	queuedJob := authoredJob
	queuedJob.Status = api.JobStatusQueued
	mf.persistence.EXPECT().StoreAuthoredJob(gomock.Any(), queuedJob).Return(nil)

	// Expect the job to be fetched from the database again:
	dbJob := persistence.Job{
		UUID:     queuedJob.JobID,
		Name:     queuedJob.Name,
		JobType:  queuedJob.JobType,
		Priority: int64(queuedJob.Priority),
		Status:   queuedJob.Status,
	}
	mf.persistence.EXPECT().FetchJob(gomock.Any(), queuedJob.JobID).Return(&dbJob, nil)

	// Expect the new job to be broadcast.
	jobUpdate := api.EventJobUpdate{
		Id:       dbJob.UUID,
		Name:     &dbJob.Name,
		Priority: int(dbJob.Priority),
		Status:   dbJob.Status,
		Type:     dbJob.JobType,
		Updated:  dbJob.UpdatedAt.Time,
	}
	mf.broadcaster.EXPECT().BroadcastNewJob(jobUpdate)

	// Do the call.
	echoCtx := mf.prepareMockedJSONRequest(submittedJob)
	requestWorkerStore(echoCtx, &worker)
	err := mf.flamenco.SubmitJob(echoCtx)
	require.NoError(t, err)
}

func TestSubmitJobWithSettings(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)
	worker := testWorker()

	submittedJob := api.SubmittedJob{
		Name:              "поднео посао",
		Type:              "test",
		Priority:          50,
		SubmitterPlatform: worker.Platform,
		Settings: &api.JobSettings{AdditionalProperties: map[string]interface{}{
			"result": "/render/frames/exploding.kittens",
		}},
		Metadata: &api.JobMetadata{AdditionalProperties: map[string]string{
			"project": "/projects/exploding-kittens",
		}},
	}

	mf.expectConvertTwoWayVariables(t,
		config.VariableAudienceWorkers,
		config.VariablePlatform(worker.Platform),
		map[string]string{
			"jobbies":  "/render/jobs",
			"frames":   "/render/frames",
			"projects": "/projects",
		},
	)

	// Same job submittedJob, but then with two-way variables injected.
	variableReplacedSettings := map[string]interface{}{
		"result": "{frames}/exploding.kittens",
	}
	variableReplacedMetadata := map[string]string{
		"project": "{projects}/exploding-kittens",
	}
	variableReplacedJob := submittedJob
	variableReplacedJob.Settings = &api.JobSettings{AdditionalProperties: variableReplacedSettings}
	variableReplacedJob.Metadata = &api.JobMetadata{AdditionalProperties: variableReplacedMetadata}

	// Expect the job compiler to be called.
	authoredJob := job_compilers.AuthoredJob{
		JobID:    "afc47568-bd9d-4368-8016-e91d945db36d",
		Name:     variableReplacedJob.Name,
		JobType:  variableReplacedJob.Type,
		Priority: variableReplacedJob.Priority,
		Status:   api.JobStatusUnderConstruction,
		Created:  mf.clock.Now(),
		Settings: variableReplacedJob.Settings.AdditionalProperties,
		Metadata: variableReplacedJob.Metadata.AdditionalProperties,
	}
	mf.jobCompiler.EXPECT().Compile(gomock.Any(), variableReplacedJob).Return(&authoredJob, nil)

	// Expect the job to be saved with 'queued' status:
	queuedJob := authoredJob
	queuedJob.Status = api.JobStatusQueued
	mf.persistence.EXPECT().StoreAuthoredJob(gomock.Any(), queuedJob).Return(nil)

	// Expect the job to be fetched from the database again:
	dbJob := persistence.Job{
		UUID:     queuedJob.JobID,
		Name:     queuedJob.Name,
		JobType:  queuedJob.JobType,
		Priority: int64(queuedJob.Priority),
		Status:   queuedJob.Status,
		Settings: []byte(`{"result": "{frames}/exploding.kittens"}`),
		Metadata: []byte(`{"project": "{projects}/exploding-kittens"}`),
	}
	mf.persistence.EXPECT().FetchJob(gomock.Any(), queuedJob.JobID).Return(&dbJob, nil)

	// Expect the new job to be broadcast.
	jobUpdate := api.EventJobUpdate{
		Id:       dbJob.UUID,
		Name:     &dbJob.Name,
		Priority: int(dbJob.Priority),
		Status:   dbJob.Status,
		Type:     dbJob.JobType,
		Updated:  dbJob.UpdatedAt.Time,
	}
	mf.broadcaster.EXPECT().BroadcastNewJob(jobUpdate)

	// Do the call.
	echoCtx := mf.prepareMockedJSONRequest(submittedJob)
	requestWorkerStore(echoCtx, &worker)
	err := mf.flamenco.SubmitJob(echoCtx)
	require.NoError(t, err)
}

func TestSubmitJobWithEtag(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	submittedJob := api.SubmittedJob{
		Name:              "поднео посао",
		Type:              "test",
		Priority:          50,
		SubmitterPlatform: "linux",
		TypeEtag:          ptr("bad etag"),
	}

	mf.jobCompiler.EXPECT().Compile(gomock.Any(), submittedJob).
		Return(nil, job_compilers.ErrJobTypeBadEtag)
	mf.expectConvertTwoWayVariables(t, config.VariableAudienceWorkers, "linux", map[string]string{}).AnyTimes()

	// Expect the job to be rejected.
	{
		echoCtx := mf.prepareMockedJSONRequest(submittedJob)
		err := mf.flamenco.SubmitJob(echoCtx)
		require.NoError(t, err)
		assertResponseAPIError(t, echoCtx,
			http.StatusPreconditionFailed, "rejecting job because its settings are outdated, refresh the job type")
	}

	// Expect the job compiler to be called.
	authoredJob := job_compilers.AuthoredJob{
		JobID:    "afc47568-bd9d-4368-8016-e91d945db36d",
		Name:     submittedJob.Name,
		JobType:  submittedJob.Type,
		Priority: submittedJob.Priority,
		Status:   api.JobStatusUnderConstruction,
		Created:  mf.clock.Now(),
	}
	mf.jobCompiler.EXPECT().Compile(gomock.Any(), gomock.Any()).Return(&authoredJob, nil)

	// Expect the job to be saved with 'queued' status:
	mf.persistence.EXPECT().StoreAuthoredJob(gomock.Any(), gomock.Any()).Return(nil)

	// Expect the job to be fetched from the database again:
	dbJob := persistence.Job{
		UUID:     authoredJob.JobID,
		Name:     authoredJob.Name,
		JobType:  authoredJob.JobType,
		Priority: int64(authoredJob.Priority),
		Status:   api.JobStatusQueued,
	}
	mf.persistence.EXPECT().FetchJob(gomock.Any(), authoredJob.JobID).Return(&dbJob, nil)

	// Expect the new job to be broadcast.
	mf.broadcaster.EXPECT().BroadcastNewJob(gomock.Any())

	{ // Expect the job with the right etag to be accepted.
		submittedJob.TypeEtag = ptr("correct etag")
		echoCtx := mf.prepareMockedJSONRequest(submittedJob)
		err := mf.flamenco.SubmitJob(echoCtx)
		require.NoError(t, err)
	}
}

func TestSubmitJobWithShamanCheckoutID(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)
	worker := testWorker()

	submittedJob := api.SubmittedJob{
		Name:              "поднео посао",
		Type:              "test",
		Priority:          50,
		SubmitterPlatform: worker.Platform,
		Storage: &api.JobStorageInfo{
			ShamanCheckoutId: ptr("Весы/Синтел"),
		},
	}

	mf.expectConvertTwoWayVariables(t,
		config.VariableAudienceWorkers,
		config.VariablePlatform(worker.Platform),
		map[string]string{},
	)

	// Expect the job compiler to be called.
	authoredJob := job_compilers.AuthoredJob{
		JobID:    "afc47568-bd9d-4368-8016-e91d945db36d",
		Name:     submittedJob.Name,
		JobType:  submittedJob.Type,
		Priority: submittedJob.Priority,
		Status:   api.JobStatusUnderConstruction,
		Created:  mf.clock.Now(),

		Storage: job_compilers.JobStorageInfo{
			ShamanCheckoutID: "Весы/Синтел",
		},
	}
	mf.jobCompiler.EXPECT().Compile(gomock.Any(), submittedJob).Return(&authoredJob, nil)

	// Expect the job to be saved with 'queued' status:
	queuedJob := authoredJob
	queuedJob.Status = api.JobStatusQueued

	mf.persistence.EXPECT().StoreAuthoredJob(gomock.Any(), queuedJob).Return(nil)

	// Expect the job to be fetched from the database again:
	dbJob := persistence.Job{
		UUID:                    queuedJob.JobID,
		Name:                    queuedJob.Name,
		JobType:                 queuedJob.JobType,
		Priority:                int64(queuedJob.Priority),
		Status:                  queuedJob.Status,
		StorageShamanCheckoutID: "Весы/Синтел",
	}
	mf.persistence.EXPECT().FetchJob(gomock.Any(), queuedJob.JobID).Return(&dbJob, nil)

	// Expect the new job to be broadcast.
	jobUpdate := api.EventJobUpdate{
		Id:       dbJob.UUID,
		Name:     &dbJob.Name,
		Priority: int(dbJob.Priority),
		Status:   dbJob.Status,
		Type:     dbJob.JobType,
		Updated:  dbJob.UpdatedAt.Time,
	}
	mf.broadcaster.EXPECT().BroadcastNewJob(jobUpdate)

	// Do the call.
	echoCtx := mf.prepareMockedJSONRequest(submittedJob)
	requestWorkerStore(echoCtx, &worker)
	err := mf.flamenco.SubmitJob(echoCtx)
	require.NoError(t, err)
}

func TestSubmitJobWithWorkerTag(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)
	worker := testWorker()

	workerTagUUID := "04435762-9dc8-4f13-80b7-643a6fa5b6fd"
	tag := persistence.WorkerTag{
		ID:          47,
		UUID:        workerTagUUID,
		Name:        "first tag",
		Description: "my first tag",
	}

	submittedJob := api.SubmittedJob{
		Name:              "поднео посао",
		Type:              "test",
		Priority:          50,
		SubmitterPlatform: worker.Platform,
		WorkerTag:         &workerTagUUID,
	}

	mf.expectConvertTwoWayVariables(t,
		config.VariableAudienceWorkers,
		config.VariablePlatform(worker.Platform),
		map[string]string{},
	)

	// Expect the job compiler to be called.
	authoredJob := job_compilers.AuthoredJob{
		JobID:         "afc47568-bd9d-4368-8016-e91d945db36d",
		WorkerTagUUID: workerTagUUID,

		Name:     submittedJob.Name,
		JobType:  submittedJob.Type,
		Priority: submittedJob.Priority,
		Status:   api.JobStatusUnderConstruction,
		Created:  mf.clock.Now(),
	}
	mf.jobCompiler.EXPECT().Compile(gomock.Any(), submittedJob).Return(&authoredJob, nil)

	// Expect the job to be saved with 'queued' status:
	queuedJob := authoredJob
	queuedJob.Status = api.JobStatusQueued
	mf.persistence.EXPECT().StoreAuthoredJob(gomock.Any(), queuedJob).Return(nil)

	// Expect the job to be fetched from the database again, including its tag.
	dbJob := persistence.Job{
		ID:        47,
		CreatedAt: mf.clock.Now(),
		UpdatedAt: sql.NullTime{Time: mf.clock.Now(), Valid: true},
		UUID:      queuedJob.JobID,
		Name:      queuedJob.Name,
		JobType:   queuedJob.JobType,
		Priority:  int64(queuedJob.Priority),
		Status:    queuedJob.Status,

		WorkerTagID: sql.NullInt64{Int64: tag.ID, Valid: true},
	}
	mf.persistence.EXPECT().FetchJob(gomock.Any(), queuedJob.JobID).Return(&dbJob, nil)
	mf.persistence.EXPECT().FetchWorkerTagByID(gomock.Any(), tag.ID).Return(tag, nil)

	// Expect the new job to be broadcast.
	jobUpdate := api.EventJobUpdate{
		Id:       dbJob.UUID,
		Name:     &dbJob.Name,
		Priority: int(dbJob.Priority),
		Status:   dbJob.Status,
		Type:     dbJob.JobType,
		Updated:  dbJob.UpdatedAt.Time,
	}
	mf.broadcaster.EXPECT().BroadcastNewJob(jobUpdate)

	// Do the call.
	echoCtx := mf.prepareMockedJSONRequest(submittedJob)
	requestWorkerStore(echoCtx, &worker)
	require.NoError(t, mf.flamenco.SubmitJob(echoCtx))

	submittedJob.SubmitterPlatform = "" // Not persisted in the database.
	assertResponseJSON(t, echoCtx, http.StatusOK, api.Job{
		SubmittedJob:      submittedJob,
		Id:                dbJob.UUID,
		Created:           dbJob.CreatedAt,
		Updated:           dbJob.UpdatedAt.Time,
		DeleteRequestedAt: nil,
		Activity:          "",
		Status:            api.JobStatusQueued,
	})
}

func TestGetJobTypeHappy(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mf := newMockedFlamenco(mockCtrl)

	// Get an existing job type.
	jt := api.AvailableJobType{
		Etag:  "some etag",
		Name:  "test-job-type",
		Label: "Test Job Type",
		Settings: []api.AvailableJobSetting{
			{Key: "setting", Type: api.AvailableJobSettingTypeString},
		},
	}
	mf.jobCompiler.EXPECT().GetJobType("test-job-type").
		Return(jt, nil)

	echoCtx := mf.prepareMockedRequest(nil)
	err := mf.flamenco.GetJobType(echoCtx, "test-job-type")
	require.NoError(t, err)

	assertResponseJSON(t, echoCtx, http.StatusOK, jt)
}

func TestGetJobTypeWithDescriptionHappy(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mf := newMockedFlamenco(mockCtrl)

	// Get an existing job type with a description.
	description := "This is a test job type"
	jt := api.AvailableJobType{
		Description: &description,
		Etag:        "some etag",
		Name:        "test-job-type",
		Label:       "Test Job Type",
		Settings: []api.AvailableJobSetting{
			{Key: "setting", Type: api.AvailableJobSettingTypeString},
		},
	}
	mf.jobCompiler.EXPECT().GetJobType("test-job-type").
		Return(jt, nil)

	echoCtx := mf.prepareMockedRequest(nil)
	err := mf.flamenco.GetJobType(echoCtx, "test-job-type")
	require.NoError(t, err)

	assertResponseJSON(t, echoCtx, http.StatusOK, jt)
}

func TestGetJobTypeUnknown(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mf := newMockedFlamenco(mockCtrl)

	// Get a non-existing job type.
	mf.jobCompiler.EXPECT().GetJobType("nonexistent-type").
		Return(api.AvailableJobType{}, job_compilers.ErrJobTypeUnknown)

	echoCtx := mf.prepareMockedRequest(nil)
	err := mf.flamenco.GetJobType(echoCtx, "nonexistent-type")
	require.NoError(t, err)
	assertResponseJSON(t, echoCtx, http.StatusNotFound, api.Error{
		Code:    http.StatusNotFound,
		Message: "no such job type known",
	})
}

func TestSubmitJobCheckWithEtag(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	submittedJob := api.SubmittedJob{
		Name:              "поднео посао",
		Type:              "test",
		Priority:          50,
		SubmitterPlatform: "linux",
		TypeEtag:          ptr("bad etag"),
	}

	mf.jobCompiler.EXPECT().Compile(gomock.Any(), submittedJob).
		Return(nil, job_compilers.ErrJobTypeBadEtag)
	mf.expectConvertTwoWayVariables(t, config.VariableAudienceWorkers, "linux", map[string]string{}).AnyTimes()

	// Expect the job to be rejected.
	{
		echoCtx := mf.prepareMockedJSONRequest(submittedJob)
		err := mf.flamenco.SubmitJobCheck(echoCtx)
		require.NoError(t, err)
		assertResponseAPIError(t, echoCtx,
			http.StatusPreconditionFailed, "rejecting job because its settings are outdated, refresh the job type")
	}

	// Expect the job compiler to be called.
	authoredJob := job_compilers.AuthoredJob{
		JobID:    "afc47568-bd9d-4368-8016-e91d945db36d",
		Name:     submittedJob.Name,
		JobType:  submittedJob.Type,
		Priority: submittedJob.Priority,
		Status:   api.JobStatusUnderConstruction,
		Created:  mf.clock.Now(),
	}
	mf.jobCompiler.EXPECT().Compile(gomock.Any(), gomock.Any()).Return(&authoredJob, nil)

	{ // Expect the job with the right etag to be accepted.
		submittedJob.TypeEtag = ptr("correct etag")
		echoCtx := mf.prepareMockedJSONRequest(submittedJob)
		err := mf.flamenco.SubmitJobCheck(echoCtx)
		require.NoError(t, err)
	}
}

func TestGetJobTypeError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mf := newMockedFlamenco(mockCtrl)

	// Get an error situation.
	mf.jobCompiler.EXPECT().GetJobType("error").
		Return(api.AvailableJobType{}, errors.New("didn't expect this"))
	echoCtx := mf.prepareMockedRequest(nil)
	err := mf.flamenco.GetJobType(echoCtx, "error")
	require.NoError(t, err)
	assertResponseAPIError(t, echoCtx, http.StatusInternalServerError, "error getting job type")
}

func TestSetJobStatus_nonexistentJob(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	jobID := "18a9b096-d77e-438c-9be2-74397038298b"
	statusUpdate := api.JobStatusChange{
		Status: api.JobStatusCancelRequested,
		Reason: "someone pushed a button",
	}

	mf.persistence.EXPECT().FetchJob(gomock.Any(), jobID).Return(nil, persistence.ErrJobNotFound)

	// Do the call.
	echoCtx := mf.prepareMockedJSONRequest(statusUpdate)
	err := mf.flamenco.SetJobStatus(echoCtx, jobID)
	require.NoError(t, err)

	assertResponseAPIError(t, echoCtx, http.StatusNotFound, "no such job")
}

func TestSetJobStatus_happy(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	jobID := "18a9b096-d77e-438c-9be2-74397038298b"
	statusUpdate := api.JobStatusChange{
		Status: api.JobStatusCancelRequested,
		Reason: "someone pushed a button",
	}
	dbJob := persistence.Job{
		UUID:   jobID,
		Name:   "test job",
		Status: api.JobStatusActive,
	}

	// Set up expectations.
	ctx := gomock.Any()
	mf.persistence.EXPECT().FetchJob(ctx, jobID).Return(&dbJob, nil)
	mf.stateMachine.EXPECT().JobStatusChange(ctx, jobID, statusUpdate.Status, "someone pushed a button")

	// Going to Cancel Requested should NOT clear the failure list.

	// Do the call.
	echoCtx := mf.prepareMockedJSONRequest(statusUpdate)
	err := mf.flamenco.SetJobStatus(echoCtx, jobID)
	require.NoError(t, err)

	assertResponseNoContent(t, echoCtx)
}

func TestSetJobPrio_nonexistentJob(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	jobID := "18a9b096-d77e-438c-9be2-74397038298b"
	prioUpdate := api.JobPriorityChange{Priority: 47}

	mf.persistence.EXPECT().
		FetchJob(moremock.ContextWithDeadline(), jobID).
		Return(nil, persistence.ErrJobNotFound)

	// Do the call.
	echoCtx := mf.prepareMockedJSONRequest(prioUpdate)
	err := mf.flamenco.SetJobStatus(echoCtx, jobID)
	require.NoError(t, err)

	assertResponseAPIError(t, echoCtx, http.StatusNotFound, "no such job")
}

func TestSetJobPrio(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	jobID := "18a9b096-d77e-438c-9be2-74397038298b"
	prioUpdate := api.JobPriorityChange{Priority: 47}
	dbJob := persistence.Job{
		UUID:     jobID,
		Name:     "test job",
		Priority: 50,
	}

	echoCtx := mf.prepareMockedJSONRequest(prioUpdate)

	// Set up expectations.
	ctx := echoCtx.Request().Context()
	mf.persistence.EXPECT().FetchJob(moremock.ContextWithDeadline(), jobID).Return(&dbJob, nil).AnyTimes()
	jobWithNewPrio := dbJob
	jobWithNewPrio.Priority = 47
	mf.persistence.EXPECT().SaveJobPriority(gomock.Not(ctx), &jobWithNewPrio)

	// Expect the change to be broadcast over SocketIO.
	expectUpdate := api.EventJobUpdate{
		Id:           dbJob.UUID,
		Name:         &dbJob.Name,
		RefreshTasks: false,
		Priority:     prioUpdate.Priority,
		Status:       dbJob.Status,
		Updated:      dbJob.UpdatedAt.Time,
	}
	mf.broadcaster.EXPECT().BroadcastJobUpdate(expectUpdate)

	err := mf.flamenco.SetJobPriority(echoCtx, jobID)
	require.NoError(t, err)

	assertResponseNoContent(t, echoCtx)
}

func TestSetJobStatusFailedToRequeueing(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	jobID := "18a9b096-d77e-438c-9be2-74397038298b"
	statusUpdate := api.JobStatusChange{
		Status: api.JobStatusRequeueing,
		Reason: "someone pushed a button",
	}
	dbJob := persistence.Job{
		UUID:   jobID,
		Name:   "test job",
		Status: api.JobStatusFailed,
	}

	// Set up expectations.
	echoCtx := mf.prepareMockedJSONRequest(statusUpdate)
	ctx := echoCtx.Request().Context()
	mf.persistence.EXPECT().FetchJob(moremock.ContextWithDeadline(), jobID).Return(&dbJob, nil)
	mf.stateMachine.EXPECT().JobStatusChange(ctx, jobID, statusUpdate.Status, "someone pushed a button")
	mf.persistence.EXPECT().ClearFailureListOfJob(ctx, &dbJob)
	mf.persistence.EXPECT().ClearJobBlocklist(ctx, &dbJob)

	// Do the call.
	err := mf.flamenco.SetJobStatus(echoCtx, jobID)
	require.NoError(t, err)

	assertResponseNoContent(t, echoCtx)
}

func TestSetTaskStatusQueued(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	jobID := "18a9b096-d77e-438c-9be2-74397038298b"
	taskID := "22a2e6e6-13a3-40e7-befd-d4ec8d97049d"
	statusUpdate := api.TaskStatusChange{
		Status: api.TaskStatusQueued,
		Reason: "someone pushed a button",
	}
	dbJob := persistence.Job{
		ID:     47,
		UUID:   jobID,
		Name:   "test job",
		Status: api.JobStatusFailed,
	}
	dbTask := persistence.Task{
		UUID:   taskID,
		Name:   "test task",
		Status: api.TaskStatusFailed,
		JobID:  dbJob.ID,
	}

	// Set up expectations.
	echoCtx := mf.prepareMockedJSONRequest(statusUpdate)
	ctx := echoCtx.Request().Context()

	taskJobWorker := persistence.TaskJobWorker{
		Task:       dbTask,
		JobUUID:    dbJob.UUID,
		WorkerUUID: "",
	}

	taskWithActivity := dbTask
	taskWithActivity.Activity = "someone pushed a button"

	mf.persistence.EXPECT().FetchTask(ctx, taskID).Return(taskJobWorker, nil)
	mf.persistence.EXPECT().SaveTaskActivity(ctx, &taskWithActivity)
	mf.stateMachine.EXPECT().TaskStatusChange(ctx, &taskWithActivity, statusUpdate.Status)
	mf.persistence.EXPECT().ClearFailureListOfTask(ctx, &taskWithActivity)

	// Do the call.
	err := mf.flamenco.SetTaskStatus(echoCtx, taskID)
	require.NoError(t, err)

	assertResponseNoContent(t, echoCtx)
}

func TestFetchTaskLogTail(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	jobID := "18a9b096-d77e-438c-9be2-74397038298b"
	taskID := "2e020eee-20f8-4e95-8dcf-65f7dfc3ebab"

	// The task can be found, but has no on-disk task log.
	// This should not cause any error, but instead be returned as "no content".
	mf.persistence.EXPECT().FetchTaskJobUUID(gomock.Any(), taskID).Return(jobID, nil)
	mf.logStorage.EXPECT().Tail(jobID, taskID).
		Return("", fmt.Errorf("wrapped error: %w", os.ErrNotExist))

	echoCtx := mf.prepareMockedRequest(nil)
	err := mf.flamenco.FetchTaskLogTail(echoCtx, taskID)
	require.NoError(t, err)
	assertResponseNoContent(t, echoCtx)

	// Check that a 204 No Content is also returned when the task log file on disk exists, but is empty.
	mf.persistence.EXPECT().FetchTaskJobUUID(gomock.Any(), taskID).Return(jobID, nil)
	mf.logStorage.EXPECT().Tail(jobID, taskID).
		Return("", fmt.Errorf("wrapped error: %w", os.ErrNotExist))

	echoCtx = mf.prepareMockedRequest(nil)
	err = mf.flamenco.FetchTaskLogTail(echoCtx, taskID)
	require.NoError(t, err)
	assertResponseNoContent(t, echoCtx)
}

func TestFetchTaskLogInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	jobID := "18a9b096-d77e-438c-9be2-74397038298b"
	taskID := "2e020eee-20f8-4e95-8dcf-65f7dfc3ebab"
	mf.persistence.EXPECT().
		FetchTaskJobUUID(gomock.Any(), taskID).
		Return(jobID, nil).
		AnyTimes()

	// The task can be found, but has no on-disk task log.
	// This should not cause any error, but instead be returned as "no content".
	mf.logStorage.EXPECT().TaskLogSize(jobID, taskID).
		Return(int64(0), fmt.Errorf("wrapped error: %w", os.ErrNotExist))

	echoCtx := mf.prepareMockedRequest(nil)
	err := mf.flamenco.FetchTaskLogInfo(echoCtx, taskID)
	require.NoError(t, err)
	assertResponseNoContent(t, echoCtx)

	// Check that a 204 No Content is also returned when the task log file on disk exists, but is empty.
	mf.logStorage.EXPECT().TaskLogSize(jobID, taskID).
		Return(int64(0), fmt.Errorf("wrapped error: %w", os.ErrNotExist))

	echoCtx = mf.prepareMockedRequest(nil)
	err = mf.flamenco.FetchTaskLogInfo(echoCtx, taskID)
	require.NoError(t, err)
	assertResponseNoContent(t, echoCtx)

	// Check that otherwise we actually get the info.
	mf.logStorage.EXPECT().TaskLogSize(jobID, taskID).Return(int64(47), nil)
	mf.logStorage.EXPECT().Filepath(jobID, taskID).Return("/path/to/job-x/test-y.txt")
	mf.localStorage.EXPECT().RelPath("/path/to/job-x/test-y.txt").Return("job-x/test-y.txt", nil)

	echoCtx = mf.prepareMockedRequest(nil)
	err = mf.flamenco.FetchTaskLogInfo(echoCtx, taskID)
	require.NoError(t, err)
	assertResponseJSON(t, echoCtx, http.StatusOK, api.TaskLogInfo{
		JobId:  jobID,
		TaskId: taskID,
		Size:   47,
		Url:    "/job-files/job-x/test-y.txt",
	})
}

func TestFetchJobLastRenderedInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	jobID := "18a9b096-d77e-438c-9be2-74397038298b"

	{
		// Last-rendered image has been processed.
		mf.lastRender.EXPECT().JobHasImage(jobID).Return(true)
		mf.lastRender.EXPECT().PathForJob(jobID).Return("/absolute/path/to/local/job/dir")
		mf.localStorage.EXPECT().RelPath("/absolute/path/to/local/job/dir").Return("relative/path", nil)
		mf.lastRender.EXPECT().ThumbSpecs().Return([]last_rendered.Thumbspec{
			{Filename: "das grosses potaat.jpg"},
			{Filename: "invisibru.jpg"},
		})

		echoCtx := mf.prepareMockedRequest(nil)
		err := mf.flamenco.FetchJobLastRenderedInfo(echoCtx, jobID)
		require.NoError(t, err)

		expectBody := api.JobLastRenderedImageInfo{
			Base:     "/job-files/relative/path",
			Suffixes: []string{"das grosses potaat.jpg", "invisibru.jpg"},
		}
		assertResponseJSON(t, echoCtx, http.StatusOK, expectBody)
	}

	{
		// No last-rendered image exists.
		mf.lastRender.EXPECT().JobHasImage(jobID).Return(false)

		echoCtx := mf.prepareMockedRequest(nil)
		err := mf.flamenco.FetchJobLastRenderedInfo(echoCtx, jobID)
		require.NoError(t, err)
		assertResponseNoContent(t, echoCtx)
	}
}

func TestFetchGlobalLastRenderedInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	jobUUID := "18a9b096-d77e-438c-9be2-74397038298b"

	{
		// No last-rendered image exists yet.
		mf.persistence.EXPECT().GetLastRenderedJobUUID(gomock.Any()).Return("", nil)

		echoCtx := mf.prepareMockedRequest(nil)
		err := mf.flamenco.FetchGlobalLastRenderedInfo(echoCtx)
		require.NoError(t, err)
		assertResponseNoContent(t, echoCtx)
	}

	{
		// Last-rendered image has been processed.
		mf.persistence.EXPECT().GetLastRenderedJobUUID(gomock.Any()).Return(jobUUID, nil)
		mf.lastRender.EXPECT().JobHasImage(jobUUID).Return(true)
		mf.lastRender.EXPECT().PathForJob(jobUUID).Return("/absolute/path/to/local/job/dir")
		mf.localStorage.EXPECT().RelPath("/absolute/path/to/local/job/dir").Return("relative/path", nil)
		mf.lastRender.EXPECT().ThumbSpecs().Return([]last_rendered.Thumbspec{
			{Filename: "das grosses potaat.jpg"},
			{Filename: "invisibru.jpg"},
		})

		echoCtx := mf.prepareMockedRequest(nil)
		err := mf.flamenco.FetchGlobalLastRenderedInfo(echoCtx)
		require.NoError(t, err)

		expectBody := api.JobLastRenderedImageInfo{
			Base:     "/job-files/relative/path",
			Suffixes: []string{"das grosses potaat.jpg", "invisibru.jpg"},
		}
		assertResponseJSON(t, echoCtx, http.StatusOK, expectBody)
	}

}

func TestDeleteJob(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	jobID := "18a9b096-d77e-438c-9be2-74397038298b"
	dbJob := persistence.Job{
		ID:     47,
		UUID:   jobID,
		Name:   "test job",
		Status: api.JobStatusFailed,
	}

	// Set up expectations.
	echoCtx := mf.prepareMockedRequest(nil)
	mf.persistence.EXPECT().FetchJob(moremock.ContextWithDeadline(), jobID).Return(&dbJob, nil)
	mf.jobDeleter.EXPECT().QueueJobDeletion(gomock.Any(), &dbJob)

	// Do the call.
	err := mf.flamenco.DeleteJob(echoCtx, jobID)
	require.NoError(t, err)

	assertResponseNoContent(t, echoCtx)
}

func TestDeleteJobMass(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	withFracionalSecs, err := time.Parse(time.RFC3339Nano, "2023-12-01T09:17:34.275+02:00")
	require.NoError(t, err)

	roundedUp, err := time.Parse(time.RFC3339Nano, "2023-12-01T09:17:35+02:00")
	require.NoError(t, err)

	body := api.DeleteJobMassJSONBody{
		LastUpdatedMax: &withFracionalSecs,
	}

	{ // Happy flow.
		echoCtx := mf.prepareMockedJSONRequest(body)
		mf.jobDeleter.EXPECT().QueueMassJobDeletion(gomock.Any(), roundedUp.UTC())

		err := mf.flamenco.DeleteJobMass(echoCtx)
		require.NoError(t, err)

		assertResponseNoContent(t, echoCtx)
	}

	{ // No jobs found.
		echoCtx := mf.prepareMockedJSONRequest(body)
		mf.jobDeleter.EXPECT().QueueMassJobDeletion(gomock.Any(), roundedUp.UTC()).
			Return(persistence.ErrJobNotFound)

		err := mf.flamenco.DeleteJobMass(echoCtx)
		require.NoError(t, err)

		assertResponseAPIError(t, echoCtx,
			http.StatusRequestedRangeNotSatisfiable,
			"no jobs modified before timestamp")
	}
}

func TestTimestampRoundUp(t *testing.T) {
	withFracionalSecs, err := time.Parse(time.RFC3339Nano, "2023-12-01T09:17:34.275+00:00")
	require.NoError(t, err)

	roundedUp, err := time.Parse(time.RFC3339Nano, "2023-12-01T09:17:35+00:00")
	require.NoError(t, err)

	assert.Equal(t, roundedUp, timestampRoundUp(withFracionalSecs))
	assert.Equal(t, roundedUp, timestampRoundUp(roundedUp))
}
