package eventbus

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/pkg/api"
)

func NewFarmStatusEvent(farmstatus api.FarmStatusReport) api.EventFarmStatus {
	return api.EventFarmStatus(farmstatus)
}

func (b *Broker) BroadcastFarmStatusEvent(event api.EventFarmStatus) {
	log.Debug().Interface("event", event).Msg("eventbus: broadcasting FarmStatus event")
	b.broadcast(TopicFarmStatus, event)
}
