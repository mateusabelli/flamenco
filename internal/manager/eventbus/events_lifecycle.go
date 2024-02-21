package eventbus

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/pkg/api"
)

func NewLifeCycleEvent(lifeCycleType api.LifeCycleEventType) api.EventLifeCycle {
	event := api.EventLifeCycle{
		Type: lifeCycleType,
	}
	return event
}

func (b *Broker) BroadcastLifeCycleEvent(event api.EventLifeCycle) {
	log.Debug().Interface("event", event).Msg("eventbus: broadcasting lifecycle event")
	b.broadcast(TopicLifeCycle, event)
}
