package api_impl

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"projects.blender.org/studio/flamenco/internal/manager/eventbus"
	"projects.blender.org/studio/flamenco/internal/manager/job_compilers"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/internal/uuid"
	"projects.blender.org/studio/flamenco/pkg/api"
	"projects.blender.org/studio/flamenco/pkg/crosspath"
)

// JobFilesURLPrefix is the URL prefix that the Flamenco API expects to serve
// the job-specific local files, i.e. the ones that are managed by
// `local_storage.StorageInfo`.
const JobFilesURLPrefix = "/job-files"

func (f *Flamenco) GetJobTypes(e echo.Context) error {
	logger := requestLogger(e)

	if f.jobCompiler == nil {
		logger.Error().Msg("Flamenco is running without job compiler")
		return sendAPIError(e, http.StatusInternalServerError, "no job types available")
	}

	logger.Debug().Msg("listing job types")
	jobTypes := f.jobCompiler.ListJobTypes()
	return e.JSON(http.StatusOK, &jobTypes)
}

func (f *Flamenco) GetJobType(e echo.Context, typeName string) error {
	logger := requestLogger(e)

	if f.jobCompiler == nil {
		logger.Error().Msg("Flamenco is running without job compiler")
		return sendAPIError(e, http.StatusInternalServerError, "no job types available")
	}

	logger.Debug().Str("typeName", typeName).Msg("getting job type")
	jobType, err := f.jobCompiler.GetJobType(typeName)
	if err != nil {
		if err == job_compilers.ErrJobTypeUnknown {
			return sendAPIError(e, http.StatusNotFound, "no such job type known")
		}
		return sendAPIError(e, http.StatusInternalServerError, "error getting job type")
	}

	return e.JSON(http.StatusOK, jobType)
}

func (f *Flamenco) SubmitJob(e echo.Context) error {
	logger := requestLogger(e)

	var job api.SubmitJobJSONRequestBody
	if err := e.Bind(&job); err != nil {
		logger.Warn().Err(err).Msg("bad request received")
		return sendAPIError(e, http.StatusBadRequest, "invalid format")
	}

	logger = logger.With().
		Str("type", job.Type).
		Str("name", job.Name).
		Logger()
	logger.Info().Msg("new Flamenco job received")

	ctx := e.Request().Context()
	authoredJob, err := f.compileSubmittedJob(ctx, logger, api.SubmittedJob(job))
	switch {
	case errors.Is(err, job_compilers.ErrJobTypeBadEtag):
		logger.Info().Err(err).Msg("rejecting submitted job because its settings are outdated, refresh the job type")
		return sendAPIError(e, http.StatusPreconditionFailed, "rejecting job because its settings are outdated, refresh the job type")
	case err != nil:
		logger.Warn().Err(err).Msg("error compiling job")
		// TODO: make this a more specific error object for this API call.
		return sendAPIError(e, http.StatusBadRequest, err.Error())
	}

	logger = logger.With().Str("job_id", authoredJob.JobID).Logger()

	submittedJob := api.SubmittedJob(job)
	initialStatus := api.JobStatusQueued
	if submittedJob.InitialStatus != nil {
		initialStatus = *submittedJob.InitialStatus
	}
	authoredJob.Status = initialStatus

	if err := f.persist.StoreAuthoredJob(ctx, *authoredJob); err != nil {
		logger.Error().Err(err).Msg("error persisting job in database")
		return sendAPIError(e, http.StatusInternalServerError, "error persisting job in database")
	}

	dbJob, err := f.persist.FetchJob(ctx, authoredJob.JobID)
	if err != nil {
		logger.Error().Err(err).Msg("unable to retrieve just-stored job from database")
		return sendAPIError(e, http.StatusInternalServerError, "error retrieving job from database")
	}

	jobUpdate := eventbus.NewJobUpdate(dbJob)
	f.broadcaster.BroadcastNewJob(jobUpdate)

	apiJob := jobDBtoAPI(dbJob)
	return e.JSON(http.StatusOK, apiJob)
}

func (f *Flamenco) SubmitJobCheck(e echo.Context) error {
	logger := requestLogger(e)

	var job api.SubmitJobCheckJSONRequestBody
	if err := e.Bind(&job); err != nil {
		logger.Warn().Err(err).Msg("bad request received")
		return sendAPIError(e, http.StatusBadRequest, "invalid format")
	}

	logger = logger.With().
		Str("type", job.Type).
		Str("name", job.Name).
		Logger()
	logger.Info().Msg("checking Flamenco job")

	ctx := e.Request().Context()
	submittedJob := api.SubmittedJob(job)
	_, err := f.compileSubmittedJob(ctx, logger, submittedJob)
	switch {
	case errors.Is(err, job_compilers.ErrJobTypeBadEtag):
		logger.Warn().Err(err).Msg("rejecting submitted job because its settings are outdated, refresh the job type")
		return sendAPIError(e, http.StatusPreconditionFailed, "rejecting job because its settings are outdated, refresh the job type")
	case err != nil:
		logger.Warn().Err(err).Msg("error compiling job")
		// TODO: make this a more specific error object for this API call.
		return sendAPIError(e, http.StatusBadRequest, err.Error())
	}

	return e.NoContent(http.StatusNoContent)
}

// compileSubmittedJob performs some processing of the job and compiles it.
func (f *Flamenco) compileSubmittedJob(ctx context.Context, logger zerolog.Logger, submittedJob api.SubmittedJob) (*job_compilers.AuthoredJob, error) {
	// Replace the special "manager" platform with the Manager's actual platform.
	if submittedJob.SubmitterPlatform == "manager" {
		submittedJob.SubmitterPlatform = runtime.GOOS
	}

	if submittedJob.TypeEtag == nil || *submittedJob.TypeEtag == "" {
		logger.Warn().Msg("job submitted without job type etag, refresh the job types in the Blender add-on")
	}

	// Before compiling the job, replace the two-way variables. This ensures all
	// the tasks also use those.
	replaceTwoWayVariables(f.config, &submittedJob)

	return f.jobCompiler.Compile(ctx, submittedJob)
}

// DeleteJob marks the job as "deletion requested" so that the job deletion
// service can actually delete it.
func (f *Flamenco) DeleteJob(e echo.Context, jobID string) error {
	logger := requestLogger(e).With().
		Str("job", jobID).
		Logger()

	dbJob, err := f.fetchJob(e, logger, jobID)
	if dbJob == nil {
		// f.fetchJob already sent a response.
		return err
	}

	logger = logger.With().
		Uint("dbID", dbJob.ID).
		Str("currentstatus", string(dbJob.Status)).
		Logger()
	logger.Info().Msg("job deletion requested")

	// All the required info is known, this can keep running even when the client
	// disconnects.
	ctx := context.Background()
	err = f.jobDeleter.QueueJobDeletion(ctx, dbJob)
	switch {
	case persistence.ErrIsDBBusy(err):
		logger.Error().AnErr("cause", err).Msg("database too busy to queue job deletion")
		return sendAPIErrorDBBusy(e, "too busy to queue job deletion, try again later")
	case err != nil:
		logger.Error().AnErr("cause", err).Msg("error queueing job deletion")
		return sendAPIError(e, http.StatusInternalServerError, "error queueing job deletion")
	default:
		return e.NoContent(http.StatusNoContent)
	}
}

func (f *Flamenco) DeleteJobWhatWouldItDo(e echo.Context, jobID string) error {
	logger := requestLogger(e).With().
		Str("job", jobID).
		Logger()

	dbJob, err := f.fetchJob(e, logger, jobID)
	if dbJob == nil {
		// f.fetchJob already sent a response.
		return err
	}

	logger = logger.With().
		Str("currentstatus", string(dbJob.Status)).
		Logger()
	logger.Info().Msg("checking what job deletion would do")

	deletionInfo := f.jobDeleter.WhatWouldBeDeleted(dbJob)
	return e.JSON(http.StatusOK, deletionInfo)
}

func timestampRoundUp(stamp time.Time) time.Time {
	truncated := stamp.Truncate(time.Second)
	if truncated == stamp {
		return stamp
	}
	return truncated.Add(time.Second)
}

func (f *Flamenco) DeleteJobMass(e echo.Context) error {
	logger := requestLogger(e)

	var settings api.DeleteJobMassJSONBody
	if err := e.Bind(&settings); err != nil {
		logger.Warn().Err(err).Msg("bad request received")
		return sendAPIError(e, http.StatusBadRequest, "invalid format")
	}

	if settings.LastUpdatedMax == nil {
		// This is the only parameter, so if this is missing, we can't do anything.
		// The parameter is optional in order to make space for future extensions.
		logger.Warn().Msg("bad request received, no 'last_updated_max' field")
		return sendAPIError(e, http.StatusBadRequest, "invalid format (no last_updated_max)")
	}

	// Round the time up to entire seconds. This makes it possible to take an
	// 'updated at' timestamp from an existing job, and delete that job + all
	// older ones.
	//
	// There might be precision differences between time representation in various
	// languages. When the to-be-deleted job has an 'updated at' timestamp at time
	// 13:14:15.100, it could get truncated to 13:14:15, which is before the
	// to-be-deleted job.
	//
	// Rounding the given timestamp up to entire seconds solves this, even though
	// it might delete too many jobs.
	lastUpdatedMax := timestampRoundUp(*settings.LastUpdatedMax)

	logger = logger.With().
		Time("lastUpdatedMax", lastUpdatedMax).
		Logger()
	logger.Info().Msg("mass deletion of jobs reqeuested")

	// All the required info is known, this can keep running even when the client
	// disconnects.
	ctx := context.Background()
	err := f.jobDeleter.QueueMassJobDeletion(ctx, lastUpdatedMax.UTC())

	switch {
	case persistence.ErrIsDBBusy(err):
		logger.Error().AnErr("cause", err).Msg("database too busy to queue job deletion")
		return sendAPIErrorDBBusy(e, "too busy to queue job deletion, try again later")
	case errors.Is(err, persistence.ErrJobNotFound):
		logger.Warn().Msg("mass job deletion: cannot find jobs modified before timestamp")
		return sendAPIError(e, http.StatusRequestedRangeNotSatisfiable, "no jobs modified before timestamp")
	case err != nil:
		logger.Error().AnErr("cause", err).Msg("error queueing job deletion")
		return sendAPIError(e, http.StatusInternalServerError, "error queueing job deletion")
	default:
		return e.NoContent(http.StatusNoContent)
	}
}

// SetJobStatus is used by the web interface to change a job's status.
func (f *Flamenco) SetJobStatus(e echo.Context, jobID string) error {
	logger := requestLogger(e).With().
		Str("job", jobID).
		Logger()

	var statusChange api.SetJobStatusJSONRequestBody
	if err := e.Bind(&statusChange); err != nil {
		logger.Warn().Err(err).Msg("bad request received")
		return sendAPIError(e, http.StatusBadRequest, "invalid format")
	}

	dbJob, err := f.fetchJob(e, logger, jobID)
	if dbJob == nil {
		// f.fetchJob already sent a response.
		return err
	}

	logger = logger.With().
		Str("currentstatus", string(dbJob.Status)).
		Str("requestedStatus", string(statusChange.Status)).
		Str("reason", statusChange.Reason).
		Logger()
	logger.Info().Msg("job status change requested")

	ctx := e.Request().Context()
	err = f.stateMachine.JobStatusChange(ctx, dbJob, statusChange.Status, statusChange.Reason)
	if err != nil {
		logger.Error().Err(err).Msg("error changing job status")
		return sendAPIError(e, http.StatusInternalServerError, "unexpected error changing job status")
	}

	// Only in this function, i.e. only when changing the job from the web
	// interface, does requeueing the job mean it should clear the failure list.
	// This is why this is implemented here, and not in the Task State Machine.
	switch statusChange.Status {
	case api.JobStatusRequeueing:
		if err := f.persist.ClearFailureListOfJob(ctx, dbJob); err != nil {
			logger.Error().Err(err).Msg("error clearing failure list")
			return sendAPIError(e, http.StatusInternalServerError, "unexpected error clearing the job's tasks' failure list")
		}
		if err := f.persist.ClearJobBlocklist(ctx, dbJob); err != nil {
			logger.Error().Err(err).Msg("error clearing failure list")
			return sendAPIError(e, http.StatusInternalServerError, "unexpected error clearing the job's tasks' failure list")
		}
	}

	return e.NoContent(http.StatusNoContent)
}

// SetJobPriority is used by the web interface to change a job's priority.
func (f *Flamenco) SetJobPriority(e echo.Context, jobID string) error {
	logger := requestLogger(e).With().
		Str("job", jobID).
		Logger()

	var prioChange api.SetJobPriorityJSONRequestBody
	if err := e.Bind(&prioChange); err != nil {
		logger.Warn().Err(err).Msg("bad request received")
		return sendAPIError(e, http.StatusBadRequest, "invalid format")
	}

	dbJob, err := f.fetchJob(e, logger, jobID)
	if dbJob == nil {
		// f.fetchJob already sent a response.
		return err
	}

	logger = logger.With().
		Str("jobName", dbJob.Name).
		Int("prioCurrent", dbJob.Priority).
		Int("prioRequested", prioChange.Priority).
		Logger()
	logger.Info().Msg("job priority change requested")

	// From here on, the request can be handled even when the client disconnects.
	bgCtx, bgCtxCancel := bgContext()
	defer bgCtxCancel()

	dbJob.Priority = prioChange.Priority
	err = f.persist.SaveJobPriority(bgCtx, dbJob)
	if err != nil {
		logger.Error().Err(err).Msg("error changing job priority")
		return sendAPIError(e, http.StatusInternalServerError, "unexpected error changing job priority")
	}

	// Broadcast this change to the SocketIO clients.
	jobUpdate := eventbus.NewJobUpdate(dbJob)
	f.broadcaster.BroadcastJobUpdate(jobUpdate)

	return e.NoContent(http.StatusNoContent)
}

// SetTaskStatus is used by the web interface to change a task's status.
func (f *Flamenco) SetTaskStatus(e echo.Context, taskID string) error {
	logger := requestLogger(e)
	ctx := e.Request().Context()

	logger = logger.With().Str("task", taskID).Logger()

	var statusChange api.SetTaskStatusJSONRequestBody
	if err := e.Bind(&statusChange); err != nil {
		logger.Warn().Err(err).Msg("bad request received")
		return sendAPIError(e, http.StatusBadRequest, "invalid format")
	}

	dbTask, err := f.persist.FetchTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, persistence.ErrTaskNotFound) {
			return sendAPIError(e, http.StatusNotFound, "no such task")
		}
		logger.Error().Err(err).Msg("error fetching task")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching task")
	}

	logger = logger.With().
		Str("currentstatus", string(dbTask.Status)).
		Str("requestedStatus", string(statusChange.Status)).
		Str("reason", statusChange.Reason).
		Logger()
	logger.Info().Msg("task status change requested")

	// Store the reason for the status change in the task's Activity.
	dbTask.Activity = statusChange.Reason
	err = f.persist.SaveTaskActivity(ctx, dbTask)
	if err != nil {
		logger.Error().Err(err).Msg("error saving reason of task status change to its activity field")
		return sendAPIError(e, http.StatusInternalServerError, "unexpected error changing task status")
	}

	// Perform the actual status change.
	err = f.stateMachine.TaskStatusChange(ctx, dbTask, statusChange.Status)
	if err != nil {
		logger.Error().Err(err).Msg("error changing task status")
		return sendAPIError(e, http.StatusInternalServerError, "unexpected error changing task status")
	}

	// Only in this function, i.e. only when changing the task from the web
	// interface, does requeueing the task mean it should clear the failure list.
	// This is why this is implemented here, and not in the Task State Machine.
	switch statusChange.Status {
	case api.TaskStatusQueued:
		if err := f.persist.ClearFailureListOfTask(ctx, dbTask); err != nil {
			logger.Error().Err(err).Msg("error clearing failure list")
			return sendAPIError(e, http.StatusInternalServerError, "unexpected error clearing the task's failure list")
		}
	}

	return e.NoContent(http.StatusNoContent)
}

func (f *Flamenco) FetchTaskLogInfo(e echo.Context, taskID string) error {
	logger := requestLogger(e)
	ctx := e.Request().Context()

	logger = logger.With().Str("task", taskID).Logger()
	if !uuid.IsValid(taskID) {
		logger.Warn().Msg("FetchTaskLogInfo: bad task ID ")
		return sendAPIError(e, http.StatusBadRequest, "bad task ID")
	}

	jobUUID, err := f.persist.FetchTaskJobUUID(ctx, taskID)
	if err != nil {
		if errors.Is(err, persistence.ErrTaskNotFound) {
			return sendAPIError(e, http.StatusNotFound, "no such task")
		}
		logger.Error().Err(err).Msg("error fetching task")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching task: %v", err)
	}
	logger = logger.With().Str("job", jobUUID).Logger()

	size, err := f.logStorage.TaskLogSize(jobUUID, taskID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Debug().Msg("task log unavailable, task has no log on disk")
			return e.NoContent(http.StatusNoContent)
		}
		logger.Error().Err(err).Msg("unable to fetch task log")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching task log: %v", err)
	}

	if size == 0 {
		logger.Debug().Msg("task log unavailable, on-disk task log is empty")
		return e.NoContent(http.StatusNoContent)
	}
	if size > math.MaxInt {
		// The OpenAPI definition just has type "integer", which translates to an
		// 'int' in Go.
		logger.Warn().
			Int64("size", size).
			Int("cappedSize", math.MaxInt).
			Msg("Task log is larger than can be stored in an int, capping the reported size. The log can still be entirely downloaded.")
		size = math.MaxInt
	}

	taskLogInfo := api.TaskLogInfo{
		TaskId: taskID,
		JobId:  jobUUID,
		Size:   int(size),
	}

	fullLogPath := f.logStorage.Filepath(jobUUID, taskID)
	relPath, err := f.localStorage.RelPath(fullLogPath)
	if err != nil {
		logger.Error().Err(err).Msg("task log is outside the manager storage, cannot construct its URL for download")
	} else {
		taskLogInfo.Url = path.Join(JobFilesURLPrefix, crosspath.ToSlash(relPath))
	}

	logger.Debug().Msg("fetched task log info")
	return e.JSON(http.StatusOK, &taskLogInfo)
}

func (f *Flamenco) FetchTaskLogTail(e echo.Context, taskID string) error {
	logger := requestLogger(e)
	ctx := e.Request().Context()

	logger = logger.With().Str("task", taskID).Logger()
	if !uuid.IsValid(taskID) {
		logger.Warn().Msg("fetchTaskLogTail: bad task ID ")
		return sendAPIError(e, http.StatusBadRequest, "bad task ID")
	}

	jobUUID, err := f.persist.FetchTaskJobUUID(ctx, taskID)
	if err != nil {
		if errors.Is(err, persistence.ErrTaskNotFound) {
			return sendAPIError(e, http.StatusNotFound, "no such task")
		}
		logger.Error().Err(err).Msg("error fetching task")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching task: %v", err)
	}
	logger = logger.With().Str("job", jobUUID).Logger()

	tail, err := f.logStorage.Tail(jobUUID, taskID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Debug().Msg("task tail unavailable, task has no log on disk")
			return e.NoContent(http.StatusNoContent)
		}
		logger.Error().Err(err).Msg("unable to fetch task log tail")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching task log tail: %v", err)
	}

	if tail == "" {
		logger.Debug().Msg("task tail unavailable, on-disk task log is empty")
		return e.NoContent(http.StatusNoContent)
	}

	logger.Debug().Msg("fetched task tail")
	return e.String(http.StatusOK, tail)
}

func (f *Flamenco) FetchJobBlocklist(e echo.Context, jobID string) error {
	if !uuid.IsValid(jobID) {
		return sendAPIError(e, http.StatusBadRequest, "job ID should be a UUID")
	}

	logger := requestLogger(e).With().Str("job", jobID).Logger()
	ctx := e.Request().Context()

	list, err := f.persist.FetchJobBlocklist(ctx, jobID)
	if err != nil {
		logger.Error().Err(err).Msg("error fetching job blocklist")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching job blocklist: %v", err)
	}

	apiList := api.JobBlocklist{}
	for _, item := range list {
		apiList = append(apiList, api.JobBlocklistEntry{
			TaskType:   item.TaskType,
			WorkerId:   item.Worker.UUID,
			WorkerName: &item.Worker.Name,
		})
	}

	return e.JSON(http.StatusOK, apiList)
}

func (f *Flamenco) RemoveJobBlocklist(e echo.Context, jobID string) error {
	if !uuid.IsValid(jobID) {
		return sendAPIError(e, http.StatusBadRequest, "job ID should be a UUID")
	}

	logger := requestLogger(e).With().Str("job", jobID).Logger()
	ctx := e.Request().Context()

	var entriesToRemove api.RemoveJobBlocklistJSONRequestBody
	if err := e.Bind(&entriesToRemove); err != nil {
		logger.Warn().Err(err).Msg("bad request received")
		return sendAPIError(e, http.StatusBadRequest, "invalid format")
	}
	if len(entriesToRemove) == 0 {
		return sendAPIError(e, http.StatusBadRequest, "empty list of blocklist entries given")
	}

	var lastErr error
	for _, entry := range entriesToRemove {
		sublogger := logger.With().
			Str("worker", entry.WorkerId).
			Str("taskType", entry.TaskType).
			Logger()
		err := f.persist.RemoveFromJobBlocklist(ctx, jobID, entry.WorkerId, entry.TaskType)
		if err != nil {
			sublogger.Error().Err(err).Msg("error removing entry from job blocklist")
			lastErr = err
			continue
		}
		sublogger.Info().Msg("removed entry from job blocklist")
	}

	if lastErr != nil {
		return sendAPIError(e, http.StatusInternalServerError,
			"error removing at least one entry from the blocklist: %v", lastErr)
	}

	return e.NoContent(http.StatusNoContent)
}

func (f *Flamenco) FetchJobLastRenderedInfo(e echo.Context, jobID string) error {
	if !uuid.IsValid(jobID) {
		return sendAPIError(e, http.StatusBadRequest, "job ID should be a UUID")
	}

	if !f.lastRender.JobHasImage(jobID) {
		return e.NoContent(http.StatusNoContent)
	}

	logger := requestLogger(e)
	info, err := f.lastRenderedInfoForJob(logger, jobID)
	if err != nil {
		logger.Error().
			Str("job", jobID).
			Err(err).
			Msg("error getting last-rendered info")
		return sendAPIError(e, http.StatusInternalServerError, "error finding last-rendered info: %v", err)
	}

	return e.JSON(http.StatusOK, info)
}

func (f *Flamenco) FetchGlobalLastRenderedInfo(e echo.Context) error {
	ctx := e.Request().Context()
	logger := requestLogger(e)

	jobUUID, err := f.persist.GetLastRenderedJobUUID(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("error getting job UUID with last-rendered image")
		return sendAPIError(e, http.StatusInternalServerError, "error finding global last-rendered info: %v", err)
	}

	if jobUUID == "" {
		return e.NoContent(http.StatusNoContent)
	}

	return f.FetchJobLastRenderedInfo(e, jobUUID)
}

func (f *Flamenco) lastRenderedInfoForJob(logger zerolog.Logger, jobUUID string) (*api.JobLastRenderedImageInfo, error) {
	basePath := f.lastRender.PathForJob(jobUUID)
	relPath, err := f.localStorage.RelPath(basePath)
	if err != nil {
		return nil, fmt.Errorf(
			"last-rendered path for job %s is %q, which is outside local storage root: %w",
			jobUUID, basePath, err)
	}

	suffixes := []string{}
	for _, spec := range f.lastRender.ThumbSpecs() {
		suffixes = append(suffixes, spec.Filename)
	}

	info := api.JobLastRenderedImageInfo{
		Base:     path.Join(JobFilesURLPrefix, relPath),
		Suffixes: suffixes,
	}
	return &info, nil
}

func jobDBtoAPI(dbJob *persistence.Job) api.Job {
	apiJob := api.Job{
		SubmittedJob: api.SubmittedJob{
			Name:     dbJob.Name,
			Priority: dbJob.Priority,
			Type:     dbJob.JobType,
		},

		Id:       dbJob.UUID,
		Created:  dbJob.CreatedAt,
		Updated:  dbJob.UpdatedAt,
		Status:   api.JobStatus(dbJob.Status),
		Activity: dbJob.Activity,
	}

	apiJob.Settings = &api.JobSettings{AdditionalProperties: dbJob.Settings}
	apiJob.Metadata = &api.JobMetadata{AdditionalProperties: dbJob.Metadata}

	if dbJob.Storage.ShamanCheckoutID != "" {
		apiJob.Storage = &api.JobStorageInfo{
			ShamanCheckoutId: &dbJob.Storage.ShamanCheckoutID,
		}
	}
	if dbJob.DeleteRequestedAt.Valid {
		apiJob.DeleteRequestedAt = &dbJob.DeleteRequestedAt.Time
	}
	if dbJob.WorkerTag != nil {
		apiJob.WorkerTag = workerTagDBtoAPI(dbJob.WorkerTag)
	}

	return apiJob
}

func taskDBtoAPI(dbTask *persistence.Task) api.Task {
	apiTask := api.Task{
		Id:       dbTask.UUID,
		Name:     dbTask.Name,
		Priority: dbTask.Priority,
		TaskType: dbTask.Type,
		Created:  dbTask.CreatedAt,
		Updated:  dbTask.UpdatedAt,
		Status:   dbTask.Status,
		Activity: dbTask.Activity,
		Commands: make([]api.Command, len(dbTask.Commands)),

		// TODO: convert this to just store dbTask.WorkerUUID.
		Worker: workerToTaskWorker(dbTask.Worker),

		JobId: dbTask.JobUUID,
	}

	if dbTask.Job != nil {
		apiTask.JobId = dbTask.Job.UUID
	}

	if !dbTask.LastTouchedAt.IsZero() {
		apiTask.LastTouched = &dbTask.LastTouchedAt
	}

	for i := range dbTask.Commands {
		apiTask.Commands[i] = commandDBtoAPI(dbTask.Commands[i])
	}

	return apiTask
}

func commandDBtoAPI(dbCommand persistence.Command) api.Command {
	return api.Command{
		Name:       dbCommand.Name,
		Parameters: dbCommand.Parameters,
	}
}

// workerToTaskWorker is nil-safe.
func workerToTaskWorker(worker *persistence.Worker) *api.TaskWorker {
	if worker == nil {
		return nil
	}
	return &api.TaskWorker{
		Id:      worker.UUID,
		Name:    worker.Name,
		Address: worker.Address,
	}
}
