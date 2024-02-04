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
	"github.com/rs/zerolog/log"
)

const (
	defaultClientID   = "flamenco"
	keepAlive         = 30 // seconds
	connectRetryDelay = 10 * time.Second

	mqttQoS = 1
)

type MQTTForwarder struct {
	config      autopaho.ClientConfig
	conn        *autopaho.ConnectionManager
	topicPrefix string

	// Context to use when publishing messages.
	ctx context.Context
}

var _ Forwarder = (*MQTTForwarder)(nil)

// MQTTClientConfig contains the MQTT client configuration.
type MQTTClientConfig struct {
	BrokerURL   string `yaml:"broker"`
	ClientID    string `yaml:"clientID"`
	TopicPrefix string `yaml:"topic_prefix"`

	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func NewMQTTForwarder(config MQTTClientConfig) *MQTTForwarder {
	config.BrokerURL = strings.TrimSpace(config.BrokerURL)
	config.ClientID = strings.TrimSpace(config.ClientID)

	if config.BrokerURL == "" {
		return nil
	}
	if config.ClientID == "" {
		config.ClientID = defaultClientID
	}

	serverURL, err := url.Parse(config.BrokerURL)
	if err != nil {
		log.Error().
			Err(err).
			Str("mqttServerURL", config.BrokerURL).
			Msg("could not parse MQTT server URL, skipping creation of MQTT client")
		return nil
	}

	client := MQTTForwarder{
		topicPrefix: config.TopicPrefix,
	}
	client.config = autopaho.ClientConfig{
		BrokerUrls:        []*url.URL{serverURL},
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
	log.Debug().Msg("mqtt client: connecting")
	conn, err := autopaho.NewConnection(ctx, m.config)
	if err != nil {
		panic(err)
	}

	m.conn = conn
	m.ctx = ctx
}

func (m *MQTTForwarder) onConnectionUp(connMgr *autopaho.ConnectionManager, connAck *paho.Connack) {
	log.Info().Msg("mqtt client: connection established")
}

func (m *MQTTForwarder) onConnectionError(err error) {
	log.Warn().AnErr("cause", err).Msg("mqtt client: could not connect to MQTT server")
}

func (m *MQTTForwarder) onClientError(err error) {
	log.Warn().AnErr("cause", err).Msg("mqtt client: server requested disconnect")
}

func (m *MQTTForwarder) onServerDisconnect(d *paho.Disconnect) {
	logEntry := log.Warn()
	if d.Properties != nil {
		logEntry = logEntry.Str("reason", d.Properties.ReasonString)
	} else {
		logEntry = logEntry.Int("reasonCode", int(d.ReasonCode))
	}
	logEntry.Msg("mqtt client: server requested disconnect")
}

func (c *MQTTForwarder) Broadcast(topic EventTopic, payload interface{}) {
	fullTopic := c.topicPrefix + string(topic)

	logger := log.With().
		Str("topic", fullTopic).
		// Interface("event", payload).
		Logger()

	asJSON, err := json.Marshal(payload)
	if err != nil {
		logger.Error().AnErr("cause", err).Interface("event", payload).
			Msg("mqtt client: could not convert event to JSON")
		return
	}

	// Publish will block so we run it in a GoRoutine.
	// TODO: might be a good idea todo this at the event broker level, rather than in this function.
	go func(topic string, msg []byte) {
		pr, err := c.conn.Publish(c.ctx, &paho.Publish{
			QoS:     mqttQoS,
			Topic:   topic,
			Payload: msg,
		})
		switch {
		case err != nil:
			logger.Error().AnErr("cause", err).Msg("mqtt client: error publishing event")
			return
		case pr.ReasonCode == 16:
			logger.Debug().Msg("mqtt client: event sent to server, but there were no subscribers")
			return
		case pr.ReasonCode != 0:
			logger.Warn().Int("reasonCode", int(pr.ReasonCode)).Msg("mqtt client: event rejected by mqtt server")
		default:
			logger.Debug().Msg("mqtt client: event sent to server")
		}
	}(fullTopic, asJSON)
}
