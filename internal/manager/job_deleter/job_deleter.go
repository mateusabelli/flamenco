// package job_deleter has functionality to delete jobs.
//
// Requesting deletion marks the job as "deletion requested" in the database.
// This is relatively fast, and persistent. After this, the job is queued for
// actual deletion by a different goroutine.
//
// At startup of the service the database is inspected and still-pending
// deletion requests are queued.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package job_deleter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/internal/manager/eventbus"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
	"projects.blender.org/studio/flamenco/pkg/shaman"
)

// jobDeletionQueueSize determines how many job deletion requests can be kept in
// memory at a time. This is variable to allow unit testing with lower limits.
var jobDeletionQueueSize = defaultJobDeletionQueueSize

const (
	defaultJobDeletionQueueSize = 100

	// jobDeletionCheckInterval determines how often the database is checked for
	// jobs that have been requested to be deleted.
	jobDeletionCheckInterval = 1 * time.Minute
)

// Service can mark jobs as "deletion requested", as well as delete those jobs
// in a background goroutine.
type Service struct {
	// Injected dependencies.
	persist           PersistenceService
	storage           Storage
	changeBroadcaster ChangeBroadcaster
	shaman            Shaman

	queue chan string // Job UUIDs to process.
}

// NewService constructs a new job deletion service.
// `shaman` can be nil if Shaman checkouts shouldn't be erased.
func NewService(
	persist PersistenceService,
	storage Storage,
	changeBroadcaster ChangeBroadcaster,
	shaman Shaman,
) *Service {
	return &Service{
		persist:           persist,
		storage:           storage,
		changeBroadcaster: changeBroadcaster,
		shaman:            shaman,

		queue: make(chan string, jobDeletionQueueSize),
	}
}

func (s *Service) QueueJobDeletion(ctx context.Context, job *persistence.Job) error {
	logger := log.With().Str("job", job.UUID).Logger()
	logger.Info().Msg("job deleter: queueing job for deletion")

	err := s.persist.RequestJobDeletion(ctx, job)
	if err != nil {
		return fmt.Errorf("requesting job deletion: %w", err)
	}

	// Broadcast that this job was queued for deleted.
	jobUpdate := eventbus.NewJobUpdate(job)
	s.changeBroadcaster.BroadcastJobUpdate(jobUpdate)

	// Let the Run() goroutine know this job is ready for deletion.
	select {
	case s.queue <- job.UUID:
		logger.Debug().Msg("job deleter: job succesfully queued for deletion")
	case <-time.After(100 * time.Millisecond):
		logger.Debug().Msg("job deleter: job deletion queue is full")
	}
	return nil
}

func (s *Service) QueueMassJobDeletion(ctx context.Context, lastUpdatedMax time.Time) error {
	logger := log.With().Time("lastUpdatedMax", lastUpdatedMax).Logger()

	uuids, err := s.persist.RequestJobMassDeletion(ctx, lastUpdatedMax)
	if err != nil {
		return fmt.Errorf("requesting mass job deletion: %w", err)
	}

	logger.Info().
		Int("numjobs", len(uuids)).
		Msg("job deleter: queueing multiple jobs for deletion")

	// Do the poking of the job deleter, and broadcasting of the job deletion, in
	// the background. The main work is done, and the rest can be done asynchronously.
	bgCtx := context.Background()
	go s.broadcastAndQueueMassJobDeletion(bgCtx, uuids, logger)

	return nil
}

func (s *Service) broadcastAndQueueMassJobDeletion(ctx context.Context, jobUUIDs []string, logger zerolog.Logger) {
	for _, uuid := range jobUUIDs {
		// Let the Run() goroutine know this job is ready for deletion.
		select {
		case s.queue <- uuid:
			logger.Debug().Msg("job deleter: job succesfully queued for deletion")
		case <-time.After(100 * time.Millisecond):
			logger.Trace().Msg("job deleter: job deletion queue is full")
		}

		// Broadcast that the jobs were queued for deletion.
		job, err := s.persist.FetchJob(ctx, uuid)
		if err != nil {
			logger.Debug().
				Str("uuid", uuid).
				Err(err).
				Msg("job deleter: unable to fetch job to send updates")
			continue
		}
		jobUpdate := eventbus.NewJobUpdate(job)
		s.changeBroadcaster.BroadcastJobUpdate(jobUpdate)
	}
}

func (s *Service) WhatWouldBeDeleted(job *persistence.Job) api.JobDeletionInfo {
	logger := log.With().Str("job", job.UUID).Logger()
	logger.Info().Msg("job deleter: checking what job deletion would do")

	return api.JobDeletionInfo{
		ShamanCheckout: s.canDeleteShamanCheckout(logger, job),
	}
}

// Run processes the queue of deletion requests. It starts by building up a
// queue of still-pending job deletions.
func (s *Service) Run(ctx context.Context) {
	s.queuePendingDeletions(ctx)

	log.Debug().Msg("job deleter: running")
	defer log.Debug().Msg("job deleter: shutting down")

	for {
		select {
		case <-ctx.Done():
			return
		case jobUUID := <-s.queue:
			s.deleteJob(ctx, jobUUID)
		case <-time.After(jobDeletionCheckInterval):
			// Inspect the database to see if there was anything marked for deletion
			// without getting into our queue. This can happen when lots of jobs are
			// queued in quick succession, as then the queue channel gets full.
			if len(s.queue) == 0 {
				s.queuePendingDeletions(ctx)
			}
		}
	}
}

func (s *Service) queuePendingDeletions(ctx context.Context) {
	log.Debug().Msg("job deleter: finding pending deletions")

	jobUUIDs, err := s.persist.FetchJobsDeletionRequested(ctx)
	if err != nil {
		log.Warn().AnErr("cause", err).Msg("job deleter: could not find jobs to be deleted in database")
		return
	}

	numDeletionsQueued := len(jobUUIDs)
queueLoop:
	for index, jobUUID := range jobUUIDs {
		select {
		case s.queue <- jobUUID:
			log.Debug().Str("job", jobUUID).Msg("job deleter: job queued for deletion")
		case <-time.After(100 * time.Millisecond):
			numRemaining := numDeletionsQueued - index
			log.Info().
				Int("deletionsQueued", len(s.queue)).
				Int("deletionsRemaining", numRemaining).
				Stringer("checkInterval", jobDeletionCheckInterval).
				Msg("job deleter: job deletion queue is full, remaining deletions will be picked up later")
			break queueLoop
		}
	}
}

func (s *Service) deleteJob(ctx context.Context, jobUUID string) error {
	logger := log.With().Str("job", jobUUID).Logger()

	logger.Debug().Msg("job deleter: starting job deletion")
	err := s.deleteShamanCheckout(ctx, logger, jobUUID)
	if err != nil {
		return err
	}

	logger.Debug().Msg("job deleter: removing logs, last-rendered images, etc.")
	if err := s.storage.RemoveJobStorage(ctx, jobUUID); err != nil {
		logger.Error().Err(err).Msg("job deleter: error removing job logs, job deletion aborted")
		return err
	}

	logger.Debug().Msg("job deleter: removing job from database")
	if err := s.persist.DeleteJob(ctx, jobUUID); err != nil {
		logger.Error().Err(err).Msg("job deleter: unable to remove job from database")
		return err
	}

	// Broadcast that this job was deleted. This only contains the UUID and the
	// "was deleted" flag, because there's nothing else left. And I don't want to
	// do a full database query for something we'll delete anyway.
	wasDeleted := true
	jobUpdate := api.EventJobUpdate{
		Id:         jobUUID,
		WasDeleted: &wasDeleted,
	}
	s.changeBroadcaster.BroadcastJobUpdate(jobUpdate)

	logger.Info().Msg("job deleter: job removal complete")

	// Request a consistency check on the database. In the past there have been
	// some issues after deleting a job.
	s.persist.RequestIntegrityCheck()

	return nil
}

func (s *Service) canDeleteShamanCheckout(logger zerolog.Logger, job *persistence.Job) bool {
	// NOTE: Keep this logic and the deleteShamanCheckout() function in sync.
	if !s.shaman.IsEnabled() {
		logger.Debug().Msg("job deleter: Shaman not enabled, cannot delete job files")
		return false
	}

	checkoutID := job.Storage.ShamanCheckoutID
	if checkoutID == "" {
		logger.Debug().Msg("job deleter: job was not created with Shaman (or before Flamenco v3.2), cannot delete job files")
		return false
	}

	return true
}

func (s *Service) deleteShamanCheckout(ctx context.Context, logger zerolog.Logger, jobUUID string) error {
	// NOTE: Keep this logic and the canDeleteShamanCheckout() function in sync.

	if !s.shaman.IsEnabled() {
		logger.Debug().Msg("job deleter: Shaman not enabled, skipping job file deletion")
		return nil
	}

	// To erase the Shaman checkout we need more info than just its UUID.
	checkoutID, err := s.persist.FetchJobShamanCheckoutID(ctx, jobUUID)
	if err != nil {
		return fmt.Errorf("unable to fetch job from database: %w", err)
	}
	if checkoutID == "" {
		logger.Info().Msg("job deleter: job was not created with Shaman (or before Flamenco v3.2), skipping job file deletion")
		return nil
	}

	err = s.shaman.EraseCheckout(checkoutID)
	switch {
	case errors.Is(err, shaman.ErrDoesNotExist):
		logger.Debug().Msg("job deleter: Shaman checkout directory does not exist, ignoring")
		return nil
	case err != nil:
		logger.Warn().Err(err).Msg("job deleter: Shaman checkout directory could not be erased")
		return err
	}

	return nil
}
