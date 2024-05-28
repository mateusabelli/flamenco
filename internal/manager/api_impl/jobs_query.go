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

	apiJob := jobDBtoAPI(dbJob)
	return e.JSON(http.StatusOK, apiJob)
}

func (f *Flamenco) QueryJobs(e echo.Context) error {
	logger := requestLogger(e)

	var jobsQuery api.QueryJobsJSONRequestBody
	if err := e.Bind(&jobsQuery); err != nil {
		logger.Warn().Err(err).Msg("bad request received")
		return sendAPIError(e, http.StatusBadRequest, "invalid format")
	}

	ctx := e.Request().Context()
	dbJobs, err := f.persist.QueryJobs(ctx, api.JobsQuery(jobsQuery))
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
		apiJobs[i] = jobDBtoAPI(dbJob)
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

	tasks, err := f.persist.QueryJobTaskSummaries(ctx, jobID)
	switch {
	case errors.Is(err, context.Canceled):
		logger.Debug().AnErr("cause", err).Msg("could not fetch job tasks, remote end probably closed connection")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching job tasks: %v", err)
	case err != nil:
		logger.Warn().Err(err).Msg("error fetching job tasks")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching job tasks: %v", err)
	}

	summaries := make([]api.TaskSummary, len(tasks))
	for i, task := range tasks {
		summaries[i] = taskDBtoSummary(task)
	}
	result := api.JobTasksSummary{
		Tasks: &summaries,
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

	// Fetch & convert the task.
	task, err := f.persist.FetchTask(ctx, taskID)
	if errors.Is(err, persistence.ErrTaskNotFound) {
		logger.Debug().Msg("non-existent task requested")
		return sendAPIError(e, http.StatusNotFound, "no such task")
	}
	if err != nil {
		logger.Warn().Err(err).Msg("error fetching task")
		return sendAPIError(e, http.StatusInternalServerError, "error fetching task")
	}
	apiTask := taskDBtoAPI(task)

	// Fetch & convert the failure list.
	failedWorkers, err := f.persist.FetchTaskFailureList(ctx, task)
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

func taskDBtoSummary(task *persistence.Task) api.TaskSummary {
	return api.TaskSummary{
		Id:       task.UUID,
		Name:     task.Name,
		Priority: task.Priority,
		Status:   task.Status,
		TaskType: task.Type,
		Updated:  task.UpdatedAt,
	}
}
