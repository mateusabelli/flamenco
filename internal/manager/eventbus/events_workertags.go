package eventbus

import (
	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// NewWorkerTagUpdate returns a partial SocketIOWorkerTagUpdate struct for the
// given worker tag. It only fills in the fields that represent the current
// state of the tag.
func NewWorkerTagUpdate(tag *persistence.WorkerTag) api.SocketIOWorkerTagUpdate {
	tagUpdate := api.SocketIOWorkerTagUpdate{
		Tag: api.WorkerTag{
			Id:          &tag.UUID,
			Name:        tag.Name,
			Description: &tag.Description,
		},
	}
	return tagUpdate
}

// NewWorkerTagDeletedUpdate returns a SocketIOWorkerTagUpdate struct that indicates
// the worker tag has been deleted.
func NewWorkerTagDeletedUpdate(tagUUID string) api.SocketIOWorkerTagUpdate {
	wasDeleted := true
	tagUpdate := api.SocketIOWorkerTagUpdate{
		Tag: api.WorkerTag{
			Id: &tagUUID,
		},
		WasDeleted: &wasDeleted,
	}
	return tagUpdate
}

func (b *Broker) BroadcastWorkerTagUpdate(workerTagUpdate api.SocketIOWorkerTagUpdate) {
	log.Debug().Interface("WorkerTagUpdate", workerTagUpdate).Msg("eventbus: broadcasting worker tag update")
	b.broadcast(TopicWorkerTagUpdate, workerTagUpdate)
}

func (b *Broker) BroadcastNewWorkerTag(workerTagUpdate api.SocketIOWorkerTagUpdate) {
	log.Debug().Interface("WorkerTagUpdate", workerTagUpdate).Msg("eventbus: broadcasting new worker tag")
	b.broadcast(TopicWorkerTagUpdate, workerTagUpdate)
}
