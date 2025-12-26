package main

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/appinfo"
	"projects.blender.org/studio/flamenco/internal/stresser"
	"projects.blender.org/studio/flamenco/internal/uuid"
)

var cliArgs struct {
	quiet, debug, trace bool

	numWorkers int
}

type WorkerInfo struct {
	Name   string
	UUID   string
	Secret string

	// TODO: merge the FakeConfig with this struct, as they track the same info.
	config *stresser.FakeConfig
}

func main() {
	parseCliArgs()

	output := zerolog.ConsoleWriter{Out: colorable.NewColorableStdout(), TimeFormat: time.RFC3339}
	log.Logger = log.Output(output)

	log.Info().
		Str("version", appinfo.ApplicationVersion).
		Str("OS", runtime.GOOS).
		Str("ARCH", runtime.GOARCH).
		Int("pid", os.Getpid()).
		Msgf("starting %v Worker", appinfo.ApplicationName)
	configLogLevel()

	mainCtx, mainCtxCancel := context.WithCancel(context.Background())

	// Handle Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		for signum := range c {
			log.Info().Str("signal", signum.String()).Msg("signal received, shutting down.")
			mainCtxCancel()
		}
	}()

	workerInfoes, err := getWorkerInfoes(cliArgs.numWorkers)
	if err != nil {
		log.Error().Err(err).Msg("could not construct worker name/uuid/secret")
		os.Exit(1)
	}

	wg := sync.WaitGroup{}
	wg.Go(func() {
		stresser.ReportStatisticsLoop(mainCtx)
	})

	for i, workerInfo := range workerInfoes[:cliArgs.numWorkers] {
		wg.Go(func() {
			config := stresser.NewFakeConfig(workerInfo.Name, workerInfo.UUID, workerInfo.Secret)
			workerInfoes[i].config = config

			client := stresser.GetFlamencoClient(mainCtx, config)

			// Stagger the startup of the workers a bit, to get a more realistic behaviour.
			time.Sleep(time.Duration(rand.Int32N(500)) * time.Millisecond)

			stresser.Run(mainCtx, client)

			log.Info().Msg("signing off at Manager")
			shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer shutdownCtxCancel()
			if _, err := client.SignOffWithResponse(shutdownCtx); err != nil {
				log.Warn().Err(err).Msg("error signing off at Manager")
			}
		})
	}
	wg.Wait()

	log.Info().Msg("stresser shutting down")

	// Store the potentially-updated worker UUID & secret in the CSV file.
	for i, info := range workerInfoes {
		if info.config == nil {
			continue
		}
		creds, _ := info.config.WorkerCredentials()
		workerInfoes[i].UUID = creds.WorkerID
		workerInfoes[i].Secret = creds.Secret
	}
	if _, err := writeWorkerInfoes(workerInfoes); err != nil {
		log.Error().Err(err).Msg("writing worker info to CSV")
		os.Exit(2)
	}
}

func parseCliArgs() {
	flag.BoolVar(&cliArgs.quiet, "quiet", false, "Only log warning-level and worse.")
	flag.BoolVar(&cliArgs.debug, "debug", false, "Enable debug-level logging.")
	flag.BoolVar(&cliArgs.trace, "trace", false, "Enable trace-level logging.")

	flag.IntVar(&cliArgs.numWorkers, "num", 1, "Number of Workers to spin up")

	flag.Parse()
}

func configLogLevel() {
	var logLevel zerolog.Level
	switch {
	case cliArgs.trace:
		logLevel = zerolog.TraceLevel
	case cliArgs.debug:
		logLevel = zerolog.DebugLevel
	case cliArgs.quiet:
		logLevel = zerolog.WarnLevel
	default:
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)
}

const workerInfoCSVPath = "stresser.csv"

func getWorkerInfoes(numWorkers int) (info []WorkerInfo, err error) {
	csvFile, err := os.Open(workerInfoCSVPath)
	switch {
	case os.IsNotExist(err):
		return constructWorkerInfoes(numWorkers)
	case err != nil:
		return nil, err
	}
	defer func() {
		closeErr := csvFile.Close()
		if err != nil {
			err = closeErr
		}
	}()

	reader := csv.NewReader(csvFile)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if err := csvFile.Close(); err != nil {
		return nil, err
	}

	switch {
	case len(records) < 2:
		// Empty file, or just the header.
		return constructWorkerInfoes(numWorkers)
	case len(records)-1 < numWorkers:
		// Not enough workers on record, expand, save, and return.
		return expandWorkerRecords(records[1:], numWorkers)
	default:
		return convertWorkerRecords(records[1:], numWorkers)
	}
}

func constructWorkerInfoes(numWorkers int) ([]WorkerInfo, error) {
	log.Info().Int("numWorkers", numWorkers).Msg("constructing new worker info list")
	infoes := make([]WorkerInfo, numWorkers)
	for i := range infoes {
		infoes[i] = newWorkerInfo(i + 1)
	}
	return writeWorkerInfoes(infoes)
}

func expandWorkerRecords(records [][]string, numWorkers int) ([]WorkerInfo, error) {
	numRecords := len(records)
	infoes, _ := convertWorkerRecords(records, numRecords)
	log.Info().
		Int("total", numWorkers).
		Int("preexisting", numRecords).
		Msg("expanding worker info list")

	for i := range numWorkers - numRecords {
		workerIndex := numRecords + i
		infoes = append(infoes, newWorkerInfo(workerIndex+1))
	}

	return writeWorkerInfoes(infoes)
}

func convertWorkerRecords(records [][]string, numWorkers int) ([]WorkerInfo, error) {
	if len(records) < numWorkers {
		panic("not enough records, use expandWorkerRecords() instead")
	}

	infoes := make([]WorkerInfo, len(records))
	for i := range infoes {
		infoes[i] = WorkerInfo{
			Name:   records[i][0],
			UUID:   records[i][1],
			Secret: records[i][2],
		}
	}
	log.Info().Int("numWorkers", len(infoes)).Msg("loaded worker info list")

	return infoes, nil
}

func writeWorkerInfoes(infoes []WorkerInfo) ([]WorkerInfo, error) {
	logger := log.With().Str("csv", workerInfoCSVPath).Logger()

	logger.Info().
		Int("numWorkers", len(infoes)).
		Msg("creating CSV file")
	csvFile, err := os.Create(workerInfoCSVPath)
	if err != nil {
		return nil, err
	}

	writer := csv.NewWriter(csvFile)
	if err := writer.Write([]string{"Name", "UUID", "Secret"}); err != nil {
		return nil, fmt.Errorf("writing header to %s: %w", workerInfoCSVPath, err)
	}
	for _, info := range infoes {
		record := []string{info.Name, info.UUID, info.Secret}
		if err := writer.Write(record); err != nil {
			return nil, fmt.Errorf("writing header to %s: %w", workerInfoCSVPath, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	if err := csvFile.Close(); err != nil {
		return nil, err
	}

	return infoes, nil
}

func newWorkerInfo(number int) WorkerInfo {
	return WorkerInfo{
		Name:   fmt.Sprintf("stresser %d", number),
		UUID:   uuid.New(),
		Secret: fmt.Sprintf("password-%d", number),
	}
}
