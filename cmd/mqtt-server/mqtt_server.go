package main

import (
	"context"
	"log/slog"
	"os"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/rs/zerolog/log"
)

const address = ":1883"

func run_mqtt_server(ctx context.Context) {

	// Create the new MQTT Server.
	options := mqtt.Options{
		Logger: slog.Default(),
	}
	server := mqtt.New(&options)

	// Allow all connections.
	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
		log.Error().Err(err).Msg("could not allow all connections, server may be unusable")
	}

	// Log incoming packets.
	hook := PacketLoggingHook{
		Logger: log.Logger,
	}
	if err := server.AddHook(&hook, nil); err != nil {
		log.Error().Err(err).Msg("could not add packet-logging hook, server may be unusable")
	}

	// Create a TCP listener on a standard port.
	tcp := listeners.NewTCP(listeners.Config{ID: "test-server", Address: address})
	tcpLogger := log.With().Str("address", address).Logger()
	if err := server.AddListener(tcp); err != nil {
		tcpLogger.Error().Err(err).Msg("listening for TCP connections")
		os.Exit(2)
	}
	tcpLogger.Info().Msg("listening for TCP connections")

	// Start the MQTT server.
	err := server.Serve()
	if err != nil {
		log.Error().Err(err).Msg("starting the server")
		os.Exit(3)
	}

	// Run server until interrupted
	<-ctx.Done()

	log.Info().Msg("shutting down server")
	server.Close()
	log.Info().Msg("shutting down")
}
