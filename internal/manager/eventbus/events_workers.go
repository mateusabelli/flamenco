package eventbus

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// NewWorkerUpdate returns a partial EventWorkerUpdate struct for the given worker.
// It only fills in the fields that represent the current state of the worker. For
// example, it omits `PreviousStatus`. The ommitted fields can be filled in by
// the caller.
func NewWorkerUpdate(worker *persistence.Worker) api.EventWorkerUpdate {
	workerUpdate := api.EventWorkerUpdate{
		Id:         worker.UUID,
		Name:       worker.Name,
		Status:     worker.Status,
		Version:    worker.Software,
		Updated:    worker.UpdatedAt,
		CanRestart: worker.CanRestart,
	}

	if worker.StatusRequested != "" {
		workerUpdate.StatusChange = &api.WorkerStatusChangeRequest{
			Status: worker.StatusRequested,
			IsLazy: worker.LazyStatusRequest,
		}
	}

	if !worker.LastSeenAt.IsZero() {
		workerUpdate.LastSeen = &worker.LastSeenAt
	}

	// TODO: add tag IDs.

	return workerUpdate
}

func (b *Broker) BroadcastNewWorker(workerUpdate api.EventWorkerUpdate) {
	if workerUpdate.PreviousStatus != nil {
		log.Warn().Interface("workerUpdate", workerUpdate).Msg("eventbus: new workers should not have a previous state")
		workerUpdate.PreviousStatus = nil
	}

	log.Debug().Interface("workerUpdate", workerUpdate).Msg("eventbus: broadcasting new worker")
	b.broadcast(TopicWorkerUpdate, workerUpdate)
}

func (b *Broker) BroadcastWorkerUpdate(workerUpdate api.EventWorkerUpdate) {
	log.Debug().Interface("workerUpdate", workerUpdate).Msg("eventbus: broadcasting worker update")
	b.broadcast(TopicWorkerUpdate, workerUpdate)
}
