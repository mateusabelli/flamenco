// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co, Sybren

package main

import (
	"encoding/json"
	"log/slog"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/storage"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/rs/zerolog"
)

type PacketLoggingHook struct {
	mqtt.HookBase
	Logger zerolog.Logger
}

// ID returns the ID of the hook.
func (h *PacketLoggingHook) ID() string           { return "payload-logger" }
func (h *PacketLoggingHook) Provides(b byte) bool { return b == mqtt.OnPacketRead }

// OnPacketRead is called when a new packet is received from a client.
func (h *PacketLoggingHook) OnPacketRead(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	if pk.FixedHeader.Type != packets.Publish {
		return pk, nil
	}

	logger := h.Logger.With().
		Str("topic", pk.TopicName).
		Uint8("qos", pk.FixedHeader.Qos).
		Uint16("id", pk.PacketID).
		Logger()

	var payload any
	err := json.Unmarshal(pk.Payload, &payload)
	if err != nil {
		logger.Info().
			AnErr("cause", err).
			Str("payload", string(pk.Payload)).
			Msg("could not unmarshal JSON")
		return pk, nil
	}

	logger.Info().
		Interface("payload", payload).
		Msg("packet")

	return pk, nil
}

func (h *PacketLoggingHook) Init(config any) error { return nil }
func (h *PacketLoggingHook) Stop() error           { return nil }
func (h *PacketLoggingHook) OnStarted()            {}
func (h *PacketLoggingHook) OnStopped()            {}

func (h *PacketLoggingHook) SetOpts(l *slog.Logger, opts *mqtt.HookOptions) {}

func (h *PacketLoggingHook) OnPacketSent(cl *mqtt.Client, pk packets.Packet, b []byte)   {}
func (h *PacketLoggingHook) OnRetainMessage(cl *mqtt.Client, pk packets.Packet, r int64) {}

func (h *PacketLoggingHook) OnQosPublish(cl *mqtt.Client, pk packets.Packet, sent int64, resends int) {
}

func (h *PacketLoggingHook) OnQosComplete(cl *mqtt.Client, pk packets.Packet) {}
func (h *PacketLoggingHook) OnQosDropped(cl *mqtt.Client, pk packets.Packet)  {}
func (h *PacketLoggingHook) OnLWTSent(cl *mqtt.Client, pk packets.Packet)     {}
func (h *PacketLoggingHook) OnRetainedExpired(filter string)                  {}
func (h *PacketLoggingHook) OnClientExpired(cl *mqtt.Client)                  {}
func (h *PacketLoggingHook) StoredClients() (v []storage.Client, err error)   { return v, nil }
func (h *PacketLoggingHook) StoredSubscriptions() (v []storage.Subscription, err error) {
	return v, nil
}
func (h *PacketLoggingHook) StoredRetainedMessages() (v []storage.Message, err error) { return v, nil }
func (h *PacketLoggingHook) StoredInflightMessages() (v []storage.Message, err error) { return v, nil }
func (h *PacketLoggingHook) StoredSysInfo() (v storage.SystemInfo, err error)         { return v, nil }
