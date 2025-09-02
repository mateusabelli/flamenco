package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	slogzerolog "github.com/samber/slog-zerolog/v2"

	"projects.blender.org/studio/flamenco/internal/appinfo"
	"projects.blender.org/studio/flamenco/pkg/sysinfo"
)

func main() {
	output := zerolog.ConsoleWriter{Out: colorable.NewColorableStdout(), TimeFormat: time.RFC3339}
	log.Logger = log.Output(output)

	osDetail, err := sysinfo.Description()
	if err != nil {
		osDetail = err.Error()
	}
	log.Info().
		Str("os", runtime.GOOS).
		Str("osDetail", osDetail).
		Str("arch", runtime.GOARCH).
		Msgf("starting %v MQTT Server", appinfo.ApplicationName)

	parseCliArgs()

	mainCtx, mainCtxCancel := context.WithCancel(context.Background())
	defer mainCtxCancel()

	// Create signals channel to run server until interrupted
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		mainCtxCancel()
	}()

	run_mqtt_server(mainCtx)
}

func parseCliArgs() {
	var quiet, debug, trace bool

	flag.BoolVar(&quiet, "quiet", false, "Only log warning-level and worse.")
	flag.BoolVar(&debug, "debug", false, "Enable debug-level logging.")
	flag.BoolVar(&trace, "trace", false, "Enable trace-level logging.")

	flag.Parse()

	var logLevel zerolog.Level
	var slogLevel slog.Level
	switch {
	case trace:
		logLevel = zerolog.TraceLevel
		slogLevel = slog.LevelDebug
	case debug:
		logLevel = zerolog.DebugLevel
		slogLevel = slog.LevelDebug
	case quiet:
		logLevel = zerolog.WarnLevel
		slogLevel = slog.LevelWarn
	default:
		logLevel = zerolog.InfoLevel
		slogLevel = slog.LevelInfo
	}
	zerolog.SetGlobalLevel(logLevel)

	// Hook up slog to zerolog.
	slogLogger := slog.New(slogzerolog.Option{
		Level:  slogLevel,
		Logger: &log.Logger}.NewZerologHandler())
	slog.SetDefault(slogLogger)
}
