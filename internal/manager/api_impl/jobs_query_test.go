// SPDX-License-Identifier: GPL-3.0-or-later
package api_impl

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

func TestQueryJobs(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	activeJob := persistence.Job{
		UUID:     "afc47568-bd9d-4368-8016-e91d945db36d",
		Name:     "работа",
		JobType:  "test",
		Priority: 50,
		Status:   api.JobStatusActive,
		Settings: persistence.StringInterfaceMap{
			"result": "/render/frames/exploding.kittens",
		},
		Metadata: persistence.StringStringMap{
			"project": "/projects/exploding-kittens",
		},
	}

	deletionRequestedAt := time.Now()
	deletionQueuedJob := persistence.Job{
		UUID:     "d912ac69-de48-48ba-8028-35d82cb41451",
		Name:     "уходить",
		JobType:  "test",
		Priority: 75,
		Status:   api.JobStatusCompleted,
		DeleteRequestedAt: sql.NullTime{
			Time:  deletionRequestedAt,
			Valid: true,
		},
	}

	echoCtx := mf.prepareMockedRequest(nil)
	ctx := echoCtx.Request().Context()
	mf.persistence.EXPECT().QueryJobs(ctx, api.JobsQuery{}).
		Return([]*persistence.Job{&activeJob, &deletionQueuedJob}, nil)

	err := mf.flamenco.QueryJobs(echoCtx)
	require.NoError(t, err)

	expectedJobs := api.JobsQueryResult{
		Jobs: []api.Job{
			{
				SubmittedJob: api.SubmittedJob{
					Name:     "работа",
					Type:     "test",
					Priority: 50,
					Settings: &api.JobSettings{AdditionalProperties: map[string]interface{}{
						"result": "/render/frames/exploding.kittens",
					}},
					Metadata: &api.JobMetadata{AdditionalProperties: map[string]string{
						"project": "/projects/exploding-kittens",
					}},
				},
				Id:     "afc47568-bd9d-4368-8016-e91d945db36d",
				Status: api.JobStatusActive,
			},
			{
				SubmittedJob: api.SubmittedJob{
					Name:     "уходить",
					Type:     "test",
					Priority: 75,
					Settings: &api.JobSettings{},
					Metadata: &api.JobMetadata{},
				},
				Id:                "d912ac69-de48-48ba-8028-35d82cb41451",
				Status:            api.JobStatusCompleted,
				DeleteRequestedAt: &deletionRequestedAt,
			},
		},
	}

	assertResponseJSON(t, echoCtx, http.StatusOK, expectedJobs)
}

func TestFetchJob(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	dbJob := persistence.Job{
		UUID:     "afc47568-bd9d-4368-8016-e91d945db36d",
		Name:     "работа",
		JobType:  "test",
		Priority: 50,
		Status:   api.JobStatusActive,
		Settings: persistence.StringInterfaceMap{
			"result": "/render/frames/exploding.kittens",
		},
		Metadata: persistence.StringStringMap{
			"project": "/projects/exploding-kittens",
		},
		WorkerTag: &persistence.WorkerTag{
			UUID:        "d86e1b84-5ee2-4784-a178-65963eeb484b",
			Name:        "Tikkie terug Kees!",
			Description: "",
		},
	}

	echoCtx := mf.prepareMockedRequest(nil)
	mf.persistence.EXPECT().FetchJob(gomock.Any(), dbJob.UUID).Return(&dbJob, nil)

	require.NoError(t, mf.flamenco.FetchJob(echoCtx, dbJob.UUID))

	expectedJob := api.Job{
		SubmittedJob: api.SubmittedJob{
			Name:     "работа",
			Type:     "test",
			Priority: 50,
			Settings: &api.JobSettings{AdditionalProperties: map[string]interface{}{
				"result": "/render/frames/exploding.kittens",
			}},
			Metadata: &api.JobMetadata{AdditionalProperties: map[string]string{
				"project": "/projects/exploding-kittens",
			}},
			WorkerTag: ptr("d86e1b84-5ee2-4784-a178-65963eeb484b"),
		},
		Id:     "afc47568-bd9d-4368-8016-e91d945db36d",
		Status: api.JobStatusActive,
	}

	assertResponseJSON(t, echoCtx, http.StatusOK, expectedJob)
}

func TestFetchTask(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	taskUUID := "19b62e32-564f-43a3-84fb-06e80ad36f16"
	workerUUID := "b5725bb3-d540-4070-a2b6-7b4b26925f94"
	jobUUID := "8b179118-0189-478a-b463-73798409898c"

	taskWorker := persistence.Worker{UUID: workerUUID, Name: "Radnik", Address: "Slapić"}

	dbTask := persistence.Task{
		Model: persistence.Model{
			ID:        327,
			CreatedAt: mf.clock.Now().Add(-30 * time.Second),
			UpdatedAt: mf.clock.Now(),
		},
		UUID:         taskUUID,
		Name:         "симпатичная задача",
		Type:         "misc",
		JobID:        0,
		Job:          &persistence.Job{UUID: jobUUID},
		Priority:     47,
		Status:       api.TaskStatusQueued,
		WorkerID:     new(uint),
		Worker:       &taskWorker,
		Dependencies: []*persistence.Task{},
		Activity:     "used in unit test",

		Commands: []persistence.Command{
			{Name: "move-directory",
				Parameters: map[string]interface{}{
					"dest": "/render/_flamenco/tests/renders/2022-04-29 Weekly/2022-04-29_140531",
					"src":  "/render/_flamenco/tests/renders/2022-04-29 Weekly/2022-04-29_140531__intermediate-2022-04-29_140531",
				}},
		},
	}

	expectAPITask := api.Task{
		Activity: "used in unit test",
		Created:  dbTask.CreatedAt,
		Id:       taskUUID,
		JobId:    jobUUID,
		Name:     "симпатичная задача",
		Priority: 47,
		Status:   api.TaskStatusQueued,
		TaskType: "misc",
		Updated:  dbTask.UpdatedAt,
		Worker:   &api.TaskWorker{Id: workerUUID, Name: "Radnik", Address: "Slapić"},

		Commands: []api.Command{
			{Name: "move-directory",
				Parameters: map[string]interface{}{
					"dest": "/render/_flamenco/tests/renders/2022-04-29 Weekly/2022-04-29_140531",
					"src":  "/render/_flamenco/tests/renders/2022-04-29 Weekly/2022-04-29_140531__intermediate-2022-04-29_140531",
				}},
		},

		FailedByWorkers: ptr([]api.TaskWorker{
			{Id: workerUUID, Name: "Radnik", Address: "Slapić"},
		}),
	}

	echoCtx := mf.prepareMockedRequest(nil)
	ctx := echoCtx.Request().Context()
	mf.persistence.EXPECT().FetchTask(ctx, taskUUID).Return(&dbTask, nil)
	mf.persistence.EXPECT().FetchTaskFailureList(ctx, &dbTask).
		Return([]*persistence.Worker{&taskWorker}, nil)

	err := mf.flamenco.FetchTask(echoCtx, taskUUID)
	require.NoError(t, err)

	assertResponseJSON(t, echoCtx, http.StatusOK, expectAPITask)
}
