package stresser

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/internal/worker"
	"projects.blender.org/studio/flamenco/pkg/api"
)

const (
	reportPeriod     = 2 * time.Second
	resetStatsPeriod = 1 * time.Minute
)

var (
	numRequests    = 0
	numFailed      = 0
	startTime      time.Time // Start of the overall stress test
	statsStartTime time.Time // Statistics are periodically reset

	mutex = sync.RWMutex{}
)

func Run(ctx context.Context, client worker.FlamencoClient) {
	// Get a task.
	task := fetchTask(ctx, client)
	if task == nil {
		log.Error().Msg("error obtaining task, shutting down stresser")
		return
	}
	logger := log.With().Str("task", task.Uuid).Logger()
	logger.Info().
		Str("job", task.Job).
		Msg("obtained task")

	// Mark the task as active.
	err := sendTaskUpdate(ctx, client, task.Uuid, api.TaskUpdate{
		Activity:       ptr("Stress testing"),
		TaskStatus:     ptr(api.TaskStatusActive),
		StepsCompleted: ptr(0),
	})
	if err != nil {
		logger.Warn().Err(err).Msg("Manager rejected task becoming active. Going to stress it anyway.")
	}

	// Do the stress test.
	for {
		if ctx.Err() != nil {
			log.Debug().Msg("stresser interrupted by context cancellation")
			return
		}

		stepsCompleted := int(time.Since(startTime))
		stepsCompleted %= task.StepsTotal
		stressBySendingTaskUpdate(ctx, client, task, &stepsCompleted)
		// stressByRequestingTask(ctx, client)
	}
}

func stressByRequestingTask(ctx context.Context, client worker.FlamencoClient) {
	increaseNumRequests()
	task := fetchTask(ctx, client)
	if task == nil {
		increaseNumFailed()
		log.Info().Msg("error obtaining task")
	}
}

//lint:ignore U1000 stressBySendingTaskUpdate is currently unused, but someone may find it useful for different kinds of stess testing.
func stressBySendingTaskUpdate(
	ctx context.Context,
	client worker.FlamencoClient,
	task *api.AssignedTask,
	stepsCompleted *int,
) {
	logLine := "This is a log-line for stress testing. It will be repeated more than once.\n"
	logToSend := strings.Repeat(logLine, 5)

	mutex.RLock()
	activity := fmt.Sprintf("stress test update %v", numRequests)
	mutex.RUnlock()

	update := api.TaskUpdate{
		Activity:       &activity,
		Log:            &logToSend,
		StepsCompleted: stepsCompleted,
	}

	increaseNumRequests()
	err := sendTaskUpdate(ctx, client, task.Uuid, update)
	switch {
	case errors.Is(err, ctx.Err()):
		// Shutting down, this is fine.
		increaseNumFailed()
		return
	case err != nil:
		log.Info().Err(err).Str("task", task.Uuid).Msg("Manager rejected task update")
		increaseNumFailed()
	}
}

func ptr[T any](value T) *T {
	return &value
}

func increaseNumRequests() {
	mutex.Lock()
	numRequests++
	mutex.Unlock()
}

func increaseNumFailed() {
	mutex.Lock()
	numFailed++
	mutex.Unlock()
}

func reportStatistics() {
	// Mark obtained-via-lock variables with an underscore, so that it's easier to see
	// that the not-locked code is only using these values.
	mutex.RLock()
	_duration := time.Since(startTime)
	_statsWindow := time.Since(statsStartTime)
	_windowInSeconds := float64(_statsWindow) / float64(time.Second)
	_reqPerSecond := float64(numRequests) / _windowInSeconds
	_numRequests := numRequests
	_numFailed := numFailed
	mutex.RUnlock()

	log.Info().
		Int("numRequests", _numRequests).
		Int("numFailed", _numFailed).
		Str("duration", _duration.Round(100*time.Millisecond).String()).
		Str("statsWindow", _statsWindow.Round(100*time.Millisecond).String()).
		Float64("requestsPerSecond", math.RoundToEven(10*_reqPerSecond)/10).
		Msg("stress progress")
}

func resetStatistics() {
	mutex.RLock()
	statsStartTime = time.Now()
	numRequests = 0
	numFailed = 0
	mutex.RUnlock()
	log.Info().Msg("statistics reset")
}

func ReportStatisticsLoop(ctx context.Context) {
	mutex.RLock()
	startTime = time.Now()
	statsStartTime = startTime
	mutex.RUnlock()

	reportTicker := time.NewTicker(reportPeriod)
	resetTicker := time.NewTicker(resetStatsPeriod)

	for {
		select {
		case <-ctx.Done():
			return
		case <-reportTicker.C:
			reportStatistics()
		case <-resetTicker.C:
			resetStatistics()
		}
	}
}
