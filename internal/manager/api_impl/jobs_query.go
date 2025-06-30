// SPDX-License-Identifier: GPL-3.0-or-later
package api_impl

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/internal/uuid"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// fetchJob fetches the job from the database, and sends the appropriate error
// to the HTTP client if it cannot. Returns `nil` in the latter case, and the
// error returned can then be returned from the Echo handler function.
func (f *Flamenco) fetchJob(e echo.Context, logger zerolog.Logger, jobID string) (*persistence.Job, error) {
	ctx, cancel := context.WithTimeout(e.Request().Context(), fetchJobTimeout)
	defer cancel()

	if !uuid.IsValid(jobID) {
		logger.Debug().Msg("invalid job ID received")
		return nil, sendAPIError(e, http.StatusBadRequest, "job ID not valid")
	}

	logger.Debug().Msg("fetching job")
	dbJob, err := f.persist.FetchJob(ctx, jobID)
	if err != nil {
		switch {
		case errors.Is(err, persistence.ErrJobNotFound):
			return nil, sendAPIError(e, http.StatusNotFound, "no such job")
		case errors.Is(err, context.DeadlineExceeded):
			logger.Error().Err(err).Msg("timeout fetching job from database")
			return nil, sendAPIError(e, http.StatusInternalServerError, "timeout fetching job from database")
		default:
			logger.Error().Err(err).Msg("error fetching job")
			return nil, sendAPIError(e, http.StatusInternalServerError, "error fetching job")
		}
	}

	return dbJob, nil
}

func (f *Flamenco) FetchJob(e echo.Context, jobID string) error {
	logger := requestLogger(e).With().
		Str("job", jobID).
		Logger()

	dbJob, err := f.fetchJob(e, logger, jobID)
	if dbJob == nil {
		// f.fetchJob already sent a response.
		return err
	}

	ctx := e.Request().Context()
	apiJob := jobDBtoAPI(ctx, f.persist, dbJob)
	return e.JSON(http.StatusOK, apiJob)
}

func (f *Flamenco) FetchJobs(e echo.Context) error {
	logger := requestLogger(e)

	ctx := e.Request().Context()
	dbJobs, err := f.persist.FetchJobs(ctx)
	switch {
	case errors.Is(err, context.Canceled):
		logger.Debug().AnErr("cause", err).Msg("could not query for jobs, remote end probably closed the connection")
		return sendAPIError(e, http.StatusInternalServerError, "error querying for jobs: %v", err)
	case err != nil:
		logger.Warn().Err(err).Msg("error querying for jobs")
		return sendAPIError(e, http.StatusInternalServerError, "error querying for jobs: %v", err)
	}

	apiJobs := make([]api.Job, len(dbJobs))
	for i, dbJob := range dbJobs {
		apiJobs[i] = jobDBtoAPI(ctx, f.persist, dbJob)
	}
	result := api.JobsQueryResult{
		Jobs: apiJobs,
	}
	return e.JSON(http.StatusOK, result)
}

func (f *Flamenco) FetchJobTasks(e echo.Context, jobID string) error {
	logger := requestLogger(e).With().
		Str("job", jobID).
		Logger()
	ctx := e.Request().Context()

	if !uuid.IsValid(jobID) {
		logger.Debug().Msg("invalid job ID received")
		return sendAPIError(e, http.StatusBadRequest, "job ID not valid")
	}

	dbSummaries, err := f.persist.QueryJobTaskSummaries(ctx, jobID)
	switch {
	case errors.Is(err, context.Canceled):
		logger.Debug().AnErr("cause", err).Msg("could not fetch job tasks, remote end probably closed connection")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching job tasks: %v", err)
	case err != nil:
		logger.Warn().Err(err).Msg("error fetching job tasks")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching job tasks: %v", err)
	}

	apiSummaries := make([]api.TaskSummary, len(dbSummaries))
	for i, dbSummary := range dbSummaries {
		apiSummaries[i] = taskSummaryDBtoAPI(dbSummary)

		// We only have the Worker UUID at this point, fetch the full worker information
		worker, err := f.persist.FetchWorker(ctx, apiSummaries[i].Worker.Id)

		switch {
		case errors.Is(err, persistence.ErrWorkerNotFound):
			// This is fine, workers can be soft-deleted from the database, and then
			// the above FetchWorker call will not return it.
		case err != nil:
			logger.Warn().Err(err).Str("worker", apiSummaries[i].Worker.Id).Msg("error fetching task worker")
			return sendAPIError(e, http.StatusInternalServerError, "error fetching task worker")
		default:
			apiSummaries[i].Worker = workerToTaskWorker(worker)
		}
	}
	result := api.JobTasksSummary{
		Tasks: &apiSummaries,
	}
	return e.JSON(http.StatusOK, result)
}

func (f *Flamenco) FetchTask(e echo.Context, taskID string) error {
	logger := requestLogger(e).With().
		Str("task", taskID).
		Logger()
	ctx := e.Request().Context()

	if !uuid.IsValid(taskID) {
		logger.Debug().Msg("invalid job ID received")
		return sendAPIError(e, http.StatusBadRequest, "job ID not valid")
	}

	// Fetch & convert the taskJobWorker.
	taskJobWorker, err := f.persist.FetchTask(ctx, taskID)
	if errors.Is(err, persistence.ErrTaskNotFound) {
		logger.Debug().Msg("non-existent task requested")
		return sendAPIError(e, http.StatusNotFound, "no such task")
	}
	if err != nil {
		logger.Warn().Err(err).Msg("error fetching task")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching task")
	}
	apiTask := taskJobWorkertoAPI(taskJobWorker)

	if taskJobWorker.WorkerUUID != "" {
		// Fetch the worker. TODO: get rid of this conversion, just include the
		// worker's UUID and let the caller fetch the worker info themselves if
		// necessary.
		taskWorker, err := f.persist.FetchWorker(ctx, taskJobWorker.WorkerUUID)
		switch {
		case errors.Is(err, persistence.ErrWorkerNotFound):
			// This is fine, workers can be soft-deleted from the database, and then
			// the above FetchWorker call will not return it.
		case err != nil:
			logger.Warn().Err(err).Str("worker", taskJobWorker.WorkerUUID).Msg("error fetching task worker")
			return sendAPIError(e, http.StatusInternalServerError, "error fetching task worker")
		default:
			apiTask.Worker = workerToTaskWorker(taskWorker)
		}
	}

	// Fetch & convert the failure list.
	failedWorkers, err := f.persist.FetchTaskFailureList(ctx, &taskJobWorker.Task)
	if err != nil {
		logger.Warn().Err(err).Msg("error fetching task failure list")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching task failure list")
	}
	failedTaskWorkers := make([]api.TaskWorker, len(failedWorkers))
	for idx, worker := range failedWorkers {
		failedTaskWorkers[idx] = *workerToTaskWorker(worker)
	}
	apiTask.FailedByWorkers = &failedTaskWorkers

	return e.JSON(http.StatusOK, apiTask)
}

func taskSummaryDBtoAPI(task persistence.TaskSummary) api.TaskSummary {
	return api.TaskSummary{
		Id:         task.UUID,
		Name:       task.Name,
		IndexInJob: int(task.IndexInJob),
		Priority:   int(task.Priority),
		Status:     task.Status,
		TaskType:   task.Type,
		Updated:    task.UpdatedAt.Time,
		Worker:     &api.TaskWorker{Id: task.WorkerUUID.String},
	}
}

func taskDBtoSummaryAPI(task persistence.Task, worker *persistence.Worker) api.TaskSummary {
	return api.TaskSummary{
		Id:         task.UUID,
		Name:       task.Name,
		IndexInJob: int(task.IndexInJob),
		Priority:   int(task.Priority),
		Status:     task.Status,
		TaskType:   task.Type,
		Updated:    task.UpdatedAt.Time,
		Worker:     workerToTaskWorker(worker),
	}
}
