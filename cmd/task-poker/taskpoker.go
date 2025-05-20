package main

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"flag"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/appinfo"
	"projects.blender.org/studio/flamenco/pkg/api"
)

var cliArgs struct {
	quiet, debug, trace bool

	jobID string
}

func updateRandomTask(ctx context.Context, apiClient *api.ClientWithResponses, job *api.Job) {
	// Fetch the current set of tasks.
	tasksResponse, err := apiClient.FetchJobTasksWithResponse(ctx, job.Id)
	if err != nil {
		log.Error().Err(err).Msg("could not fetch tasks of job")
		return
	}
	if tasksResponse.StatusCode() != http.StatusOK {
		log.Error().
			Str("jobID", job.Id).
			Int("status", tasksResponse.StatusCode()).
			Msg("could not fetch tasks of job")
	}
	tasks := tasksResponse.JSON200.Tasks
	if tasks == nil {
		log.Warn().Msg("job has no tasks, nothing to do")
		return
	}
	log.Debug().Int("numTasks", len(*tasks)).Msg("found tasks")

	taskIndex := rand.IntN(len(*tasks))
	task := (*tasks)[taskIndex]

	logger := log.With().
		Int("taskIndex", taskIndex).
		Str("taskName", task.Name).
		Str("currentStatus", string(task.Status)).
		Logger()
	logger.Info().Msg("going to poke at task")

	// Find a suitable new status.
	var newStatus api.TaskStatus
	switch task.Status {
	case api.TaskStatusQueued:
		newStatus = api.TaskStatusPaused
	case api.TaskStatusPaused:
		newStatus = api.TaskStatusQueued
	}
	if newStatus == "" {
		logger.Info().Msg("could not find a new status for this task, ignoring")
		return
	}

	logger = logger.With().Str("newStatus", string(newStatus)).Logger()
	logger.Info().Msg("updating task status")

	resp, err := apiClient.SetTaskStatusWithResponse(ctx, task.Id, api.SetTaskStatusJSONRequestBody{
		Reason: "Randomized the task status",
		Status: newStatus,
	})

	if err != nil {
		logger.Error().Err(err).Msg("could not send task update")
		return
	}
	if resp.StatusCode() != http.StatusNoContent {
		logger.Error().
			Int("status", tasksResponse.StatusCode()).
			Msg("could not update task to new status")
	}
}

func main() {
	parseCliArgs()

	output := zerolog.ConsoleWriter{Out: colorable.NewColorableStdout(), TimeFormat: time.RFC3339}
	log.Logger = log.Output(output)

	log.Info().
		Str("version", appinfo.ApplicationVersion).
		Str("OS", runtime.GOOS).
		Str("ARCH", runtime.GOARCH).
		Int("pid", os.Getpid()).
		Msgf("starting %v Task Poker", appinfo.ApplicationName)
	configLogLevel()

	mainCtx, mainCtxCancel := context.WithCancel(context.Background())

	// Handle Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		for signum := range c {
			log.Info().Str("signal", signum.String()).Msg("signal received, shutting down.")
			mainCtxCancel()
		}
	}()

	// Construct an API client.
	apiClient, err := api.NewClientWithResponses("http://localhost:8080/")
	if err != nil {
		log.Fatal().Err(err).Msg("error creating client")
	}
	jobResponse, err := apiClient.FetchJobWithResponse(mainCtx, cliArgs.jobID)
	if err != nil {
		log.Fatal().Err(err).Str("jobID", cliArgs.jobID).Msg("could not find this job")
	}
	if jobResponse.StatusCode() != http.StatusOK {
		log.Fatal().
			Err(err).
			Str("jobID", cliArgs.jobID).
			Int("status", jobResponse.StatusCode()).
			Msg("could not fetch this job")
	}
	log.Info().Str("name", jobResponse.JSON200.Name).Msg("going to poke tasks of this job")

	ticker := time.NewTicker(1 * time.Second)
mainloop:
	for {
		select {
		case <-mainCtx.Done():
			break mainloop
		case <-ticker.C:
			updateRandomTask(mainCtx, apiClient, jobResponse.JSON200)
		}
	}

	log.Info().Msg("task poker shutting down")
}

func parseCliArgs() {
	flag.BoolVar(&cliArgs.quiet, "quiet", false, "Only log warning-level and worse.")
	flag.BoolVar(&cliArgs.debug, "debug", false, "Enable debug-level logging.")
	flag.BoolVar(&cliArgs.trace, "trace", false, "Enable trace-level logging.")

	flag.StringVar(&cliArgs.jobID, "job", "", "UUID of the job to update")

	flag.Parse()
}

func configLogLevel() {
	var logLevel zerolog.Level
	switch {
	case cliArgs.trace:
		logLevel = zerolog.TraceLevel
	case cliArgs.debug:
		logLevel = zerolog.DebugLevel
	case cliArgs.quiet:
		logLevel = zerolog.WarnLevel
	default:
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)
}
