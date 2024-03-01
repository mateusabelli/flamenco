// package farmstatus provides a status indicator for the entire Flamenco farm.
package farmstatus

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/internal/manager/eventbus"
	"projects.blender.org/studio/flamenco/pkg/api"
	"projects.blender.org/studio/flamenco/pkg/website"
)

const (
	// pollWait determines how often the persistence layer is queried to get the
	// counts & statuses of workers and jobs.
	//
	// Note that this indicates the time between polls, so between a poll
	// operation being done, and the next one starting.
	pollWait = 5 * time.Second
)

// Service keeps track of the overall farm status.
type Service struct {
	persist  PersistenceService
	eventbus EventBus

	mutex      sync.Mutex
	lastReport api.FarmStatusReport
}

func NewService(persist PersistenceService, eventbus EventBus) *Service {
	return &Service{
		persist:  persist,
		eventbus: eventbus,
		mutex:    sync.Mutex{},
		lastReport: api.FarmStatusReport{
			Status: api.FarmStatusStarting,
		},
	}
}

// Run the farm status polling loop.
func (s *Service) Run(ctx context.Context) {
	log.Debug().Msg("farm status: polling service running")
	defer log.Debug().Msg("farm status: polling service stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollWait):
			s.poll(ctx)
		}
	}
}

// Report returns the last-known farm status report.
//
// It is updated every few seconds, from the Run() function.
func (s *Service) Report() api.FarmStatusReport {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.lastReport
}

// updateStatusReport updates the last status report in a thread-safe way.
// It returns whether the report changed.
func (s *Service) updateStatusReport(report api.FarmStatusReport) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	reportChanged := s.lastReport != report
	s.lastReport = report

	return reportChanged
}

func (s *Service) poll(ctx context.Context) {
	report := s.checkFarmStatus(ctx)
	if report == nil {
		// Already logged, just keep the last known log around for querying.
		return
	}

	reportChanged := s.updateStatusReport(*report)
	if reportChanged {
		event := eventbus.NewFarmStatusEvent(s.lastReport)
		s.eventbus.BroadcastFarmStatusEvent(event)
	}
}

// checkFarmStatus checks the farm status by querying the peristence layer.
// This function does not return an error, but instead logs them as warnings and returns nil.
func (s *Service) checkFarmStatus(ctx context.Context) *api.FarmStatusReport {
	log.Trace().Msg("farm status: checking the farm status")
	startTime := time.Now()

	defer func() {
		duration := time.Since(startTime)
		log.Debug().Stringer("duration", duration).Msg("farm status: checked the farm status")
	}()

	workerStatuses, err := s.persist.SummarizeWorkerStatuses(ctx)
	if err != nil {
		logDBError(err, "farm status: could not summarize worker statuses")
		return nil
	}

	// Check some worker statuses first. When there are no workers and the farm is
	// inoperative, there is little use in checking jobs. At least for now. Maybe
	// later we want to have some info in the reported status that indicates a
	// more pressing matter (as in, inoperative AND a job is queued).

	// Check: inoperative
	if len(workerStatuses) == 0 || allIn(workerStatuses, api.WorkerStatusOffline, api.WorkerStatusError) {
		return &api.FarmStatusReport{
			Status: api.FarmStatusInoperative,
		}
	}

	jobStatuses, err := s.persist.SummarizeJobStatuses(ctx)
	if err != nil {
		logDBError(err, "farm status: could not summarize job statuses")
		return nil
	}

	anyJobActive := jobStatuses[api.JobStatusActive] > 0
	anyJobQueued := jobStatuses[api.JobStatusQueued] > 0
	isWorkAvailable := anyJobActive || anyJobQueued

	anyWorkerAwake := workerStatuses[api.WorkerStatusAwake] > 0
	anyWorkerAsleep := workerStatuses[api.WorkerStatusAsleep] > 0
	allWorkersAsleep := !anyWorkerAwake && anyWorkerAsleep

	report := api.FarmStatusReport{}
	switch {
	case anyJobActive && anyWorkerAwake:
		// - "active" # Actively working on jobs.
		report.Status = api.FarmStatusActive
	case isWorkAvailable:
		// - "waiting" # Work to be done, but there is no worker awake.
		report.Status = api.FarmStatusWaiting
	case !isWorkAvailable && allWorkersAsleep:
		// - "asleep" # Farm is idle, and all workers are asleep.
		report.Status = api.FarmStatusAsleep
	case !isWorkAvailable:
		// - "idle" # Farm could be active, but has no work to do.
		report.Status = api.FarmStatusIdle
	default:
		log.Warn().
			Interface("workerStatuses", workerStatuses).
			Interface("jobStatuses", jobStatuses).
			Msgf("farm status: unexpected configuration of worker and job statuses, please report this at %s", website.BugReportURL)
		report.Status = api.FarmStatusUnknown
	}

	return &report
}

func logDBError(err error, message string) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		log.Warn().Msg(message + " (it took too long)")
	case errors.Is(err, context.Canceled):
		log.Debug().Msg(message + " (Flamenco is shutting down)")
	default:
		log.Warn().AnErr("cause", err).Msg(message)
	}
}

func allIn[T comparable](statuses map[T]int, shouldBeIn ...T) bool {
	for status, count := range statuses {
		if count == 0 {
			continue
		}

		if !slices.Contains(shouldBeIn, status) {
			return false
		}
	}
	return true
}
