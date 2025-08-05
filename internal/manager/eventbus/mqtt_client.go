package eventbus

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/pkg/api"
)

const (
	MQTTDefaultTopicPrefix = "flamenco"
	MQTTDefaultClientID    = "flamenco"

	keepAlive         = 30 // seconds
	connectRetryDelay = 10 * time.Second

	mqttQoS       = 1  // QoS field for MQTT events.
	mqttQueueSize = 10 // How many events are queued when there is no connection to an MQTT broker.
)

type MQTTForwarder struct {
	config      autopaho.ClientConfig
	conn        *autopaho.ConnectionManager
	topicPrefix string

	// Context to use when publishing messages.
	ctx context.Context

	queue       chan mqttQueuedMessage
	queueCancel context.CancelFunc
}

var _ Forwarder = (*MQTTForwarder)(nil)

// MQTTClientConfig contains the MQTT client configuration.
type MQTTClientConfig struct {
	BrokerURL   string `json:"broker" yaml:"broker"`
	ClientID    string `json:"clientID" yaml:"clientID"`
	TopicPrefix string `json:"topic_prefix" yaml:"topic_prefix"`

	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

type mqttQueuedMessage struct {
	topic   string
	payload []byte
}

func NewMQTTForwarder(config MQTTClientConfig) *MQTTForwarder {
	config.BrokerURL = strings.TrimSpace(config.BrokerURL)
	config.ClientID = strings.TrimSpace(config.ClientID)

	if config.BrokerURL == "" {
		return nil
	}
	if config.ClientID == "" {
		config.ClientID = MQTTDefaultClientID
	}

	brokerURL, err := url.Parse(config.BrokerURL)
	if err != nil {
		log.Error().
			Err(err).
			Str("brokerURL", config.BrokerURL).
			Msg("mqtt client: could not parse MQTT broker URL, skipping creation of MQTT client")
		return nil
	}

	client := MQTTForwarder{
		topicPrefix: config.TopicPrefix,
		queue:       make(chan mqttQueuedMessage, mqttQueueSize),
	}
	client.config = autopaho.ClientConfig{
		BrokerUrls:        []*url.URL{brokerURL},
		KeepAlive:         keepAlive,
		ConnectRetryDelay: connectRetryDelay,
		OnConnectionUp:    client.onConnectionUp,
		OnConnectError:    client.onConnectionError,
		Debug:             paho.NOOPLogger{},
		ClientConfig: paho.ClientConfig{
			ClientID:           config.ClientID,
			OnClientError:      client.onClientError,
			OnServerDisconnect: client.onServerDisconnect,
		},
	}
	client.config.SetUsernamePassword(config.Username, []byte(config.Password))
	return &client
}

func (m *MQTTForwarder) Connect(ctx context.Context) {
	m.logger().Debug().Msg("mqtt client: connecting to broker")
	conn, err := autopaho.NewConnection(ctx, m.config)
	if err != nil {
		panic(err)
	}

	m.conn = conn
	m.ctx = ctx
}

func (m *MQTTForwarder) onConnectionUp(connMgr *autopaho.ConnectionManager, connAck *paho.Connack) {
	m.logger().Info().Msg("mqtt client: connection established")

	queueCtx, queueCtxCancel := context.WithCancel(m.ctx)
	m.queueCancel = queueCtxCancel
	go m.queueRunner(queueCtx)
}

func (m *MQTTForwarder) onConnectionError(err error) {
	m.logger().Warn().AnErr("cause", err).Msg("mqtt client: could not connect to MQTT broker")
}

func (m *MQTTForwarder) onClientError(err error) {
	m.logger().Warn().AnErr("cause", err).Msg("mqtt client: broker requested disconnect")
}

func (m *MQTTForwarder) onServerDisconnect(d *paho.Disconnect) {
	if m.queueCancel != nil {
		m.queueCancel()
	}

	logEntry := m.logger().Warn()
	if d.Properties != nil {
		logEntry = logEntry.Str("reason", d.Properties.ReasonString)
	} else {
		logEntry = logEntry.Int("reasonCode", int(d.ReasonCode))
	}
	logEntry.Msg("mqtt client: broker requested disconnect")
}

func (m *MQTTForwarder) queueRunner(queueRunnerCtx context.Context) {
	m.logger().Debug().Msg("mqtt client: starting queue runner")
	defer m.logger().Debug().Msg("mqtt client: stopping queue runner")

	for {
		select {
		case <-queueRunnerCtx.Done():
			return
		case message := <-m.queue:
			m.sendEvent(message)
		}
	}
}

func (m *MQTTForwarder) Broadcast(topic EventTopic, payload interface{}) {
	if _, ok := payload.(api.EventTaskLogUpdate); ok {
		// Task log updates aren't sent through MQTT, as that can generate a lot of traffic.
		return
	}

	fullTopic := m.topicPrefix + string(topic)

	asJSON, err := json.Marshal(payload)
	if err != nil {
		m.logger().Error().
			Str("topic", fullTopic).
			AnErr("cause", err).
			Interface("event", payload).
			Msg("mqtt client: could not convert event to JSON")
		return
	}

	// Queue the message, if we can.
	message := mqttQueuedMessage{
		topic:   fullTopic,
		payload: asJSON,
	}
	select {
	case m.queue <- message:
		// All good, message is queued.
	default:
		m.logger().Error().
			Str("topic", fullTopic).
			Msg("mqtt client: could not send event, queue is full")
	}
}

func (m *MQTTForwarder) sendEvent(message mqttQueuedMessage) {
	logger := m.logger().With().
		Str("topic", message.topic).
		Logger()

	pr, err := m.conn.Publish(m.ctx, &paho.Publish{
		QoS:     mqttQoS,
		Topic:   message.topic,
		Payload: message.payload,
	})
	switch {
	case err != nil:
		logger.Error().AnErr("cause", err).Msg("mqtt client: error publishing event")
		return
	case pr.ReasonCode == 16:
		logger.Debug().Msg("mqtt client: event sent to broker, but there were no subscribers")
		return
	case pr.ReasonCode != 0:
		logger.Warn().Int("reasonCode", int(pr.ReasonCode)).Msg("mqtt client: event rejected by mqtt broker")
	default:
		logger.Debug().Msg("mqtt client: event sent to broker")
	}
}

func (m *MQTTForwarder) logger() *zerolog.Logger {
	logCtx := log.With()

	if len(m.config.BrokerUrls) > 0 {
		// Assumption: there's no more than one broker URL.
		logCtx = logCtx.Stringer("broker", m.config.BrokerUrls[0])
	}

	logger := logCtx.Logger()
	return &logger
}
