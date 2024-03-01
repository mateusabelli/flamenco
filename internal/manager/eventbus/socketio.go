package eventbus

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"errors"
	"fmt"
	"reflect"

	gosocketio "github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/internal/uuid"
	"projects.blender.org/studio/flamenco/pkg/api"
	"projects.blender.org/studio/flamenco/pkg/website"
)

type SocketIOEventType string

const (
	SIOEventSubscription SocketIOEventType = "/subscription" // clients send api.SocketIOSubscription
)

var socketIOEventTypes = map[string]string{
	reflect.TypeOf(api.EventJobUpdate{}).Name():          "/jobs",
	reflect.TypeOf(api.EventTaskUpdate{}).Name():         "/task",
	reflect.TypeOf(api.EventLastRenderedUpdate{}).Name(): "/last-rendered",
	reflect.TypeOf(api.EventTaskLogUpdate{}).Name():      "/tasklog",
	reflect.TypeOf(api.EventWorkerTagUpdate{}).Name():    "/workertags",
	reflect.TypeOf(api.EventWorkerUpdate{}).Name():       "/workers",
}

// SocketIOForwarder is an event forwarder via SocketIO.
type SocketIOForwarder struct {
	sockserv *gosocketio.Server
}

var _ Forwarder = (*SocketIOForwarder)(nil)

type Message struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

func NewSocketIOForwarder() *SocketIOForwarder {
	siof := SocketIOForwarder{
		sockserv: gosocketio.NewServer(transport.GetDefaultWebsocketTransport()),
	}
	siof.registerSIOEventHandlers()
	return &siof
}

func (s *SocketIOForwarder) RegisterHandlers(router *echo.Echo) {
	router.Any("/socket.io/", echo.WrapHandler(s.sockserv))
}

func (s *SocketIOForwarder) Broadcast(topic EventTopic, payload interface{}) {
	// SocketIO has a concept of 'event types'. MQTT doesn't have this, and thus the Flamenco event
	// system doesn't rely on it. We use the payload type name as event type.
	payloadType := reflect.TypeOf(payload).Name()

	eventType, ok := socketIOEventTypes[payloadType]
	if !ok {
		log.Error().
			Str("topic", string(topic)).
			Str("payloadType", payloadType).
			Interface("event", payload).
			Msgf("socketIO: payload type does not have an event type, please copy-paste this message into a bug report at %s", website.BugReportURL)
		return
	}

	log.Debug().
		Str("topic", string(topic)).
		Str("eventType", eventType).
		// Interface("event", payload).
		Msg("socketIO: broadcasting message")
	s.sockserv.BroadcastTo(string(topic), eventType, payload)
}

func (s *SocketIOForwarder) registerSIOEventHandlers() {
	log.Debug().Msg("initialising SocketIO")

	sio := s.sockserv
	// the sio.On() and c.Join() calls only return an error when there is no
	// server connected to them, but that's not possible with our setup.
	// Errors are explicitly silenced (by assigning to _) to reduce clutter.

	// socket connection
	_ = sio.On(gosocketio.OnConnection, func(c *gosocketio.Channel) {
		logger := sioLogger(c)
		logger.Debug().Msg("socketIO: connected")
	})

	// socket disconnection
	_ = sio.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
		logger := sioLogger(c)
		logger.Debug().Msg("socketIO: disconnected")
	})

	_ = sio.On(gosocketio.OnError, func(c *gosocketio.Channel) {
		logger := sioLogger(c)
		logger.Warn().Msg("socketIO: socketio error")
	})

	s.registerRoomEventHandlers()
}

func sioLogger(c *gosocketio.Channel) zerolog.Logger {
	logger := log.With().
		Str("clientID", c.Id()).
		Str("remoteAddr", c.Ip()).
		Logger()
	return logger
}

func (s *SocketIOForwarder) registerRoomEventHandlers() {
	_ = s.sockserv.On(string(SIOEventSubscription), s.handleRoomSubscription)
}

func (s *SocketIOForwarder) handleRoomSubscription(c *gosocketio.Channel, subs api.SocketIOSubscription) string {
	logger := sioLogger(c)
	logCtx := logger.With().
		Str("op", string(subs.Op)).
		Str("type", string(subs.Type))
	if subs.Uuid != nil {
		logCtx = logCtx.Str("uuid", string(*subs.Uuid))
	}
	logger = logCtx.Logger()

	if subs.Uuid != nil && !uuid.IsValid(*subs.Uuid) {
		logger.Warn().Msg("socketIO: invalid UUID, ignoring subscription request")
		return "invalid UUID, ignoring request"
	}

	var err error
	switch subs.Type {
	case api.SocketIOSubscriptionTypeAllJobs:
		err = s.subUnsub(c, TopicJobUpdate, subs.Op)
	case api.SocketIOSubscriptionTypeAllWorkers:
		err = s.subUnsub(c, TopicWorkerUpdate, subs.Op)
	case api.SocketIOSubscriptionTypeAllLastRendered:
		err = s.subUnsub(c, TopicLastRenderedImage, subs.Op)
	case api.SocketIOSubscriptionTypeAllWorkerTags:
		err = s.subUnsub(c, TopicWorkerTagUpdate, subs.Op)
	case api.SocketIOSubscriptionTypeJob:
		if subs.Uuid == nil {
			logger.Warn().Msg("socketIO: trying to (un)subscribe to job without UUID")
			return "operation on job requires a UUID"
		}
		logger.Trace().Msg("socketio: sub subscription, also going to do last-rendered for that job")
		err = s.subUnsub(c, topicForJob(*subs.Uuid), subs.Op)
		if err == nil {
			err = s.subUnsub(c, topicForJobLastRendered(*subs.Uuid), subs.Op)
		}
	case api.SocketIOSubscriptionTypeTasklog:
		if subs.Uuid == nil {
			logger.Warn().Msg("socketIO: trying to (un)subscribe to task without UUID")
			return "operation on task requires a UUID"
		}
		err = s.subUnsub(c, topicForJob(*subs.Uuid), subs.Op)
	default:
		logger.Warn().Msg("socketIO: unknown subscription type, ignoring")
		return "unknown subscription type, ignoring request"
	}

	if err != nil {
		logger.Warn().Err(err).Msg("socketIO: performing subscription operation")
		return fmt.Sprintf("unable to perform subscription operation: %v", err)
	}

	logger.Debug().Msg("socketIO: subscription")
	return "ok"
}

func (s *SocketIOForwarder) subUnsub(
	c *gosocketio.Channel,
	topic EventTopic,
	operation api.SocketIOSubscriptionOperation,
) error {
	room := string(topic)
	switch operation {
	case api.SocketIOSubscriptionOperationSubscribe:
		return c.Join(room)
	case api.SocketIOSubscriptionOperationUnsubscribe:
		return c.Leave(room)
	default:
		return errors.New("invalid subscription operation, ignoring request")
	}
}
