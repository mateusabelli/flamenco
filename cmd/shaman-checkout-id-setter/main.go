package main

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/appinfo"
	"projects.blender.org/studio/flamenco/internal/manager/config"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
)

func main() {
	output := zerolog.ConsoleWriter{Out: colorable.NewColorableStdout(), TimeFormat: time.RFC3339}
	log.Logger = log.Output(output)
	log.Info().
		Str("version", appinfo.ApplicationVersion).
		Str("git", appinfo.ApplicationGitHash).
		Str("releaseCycle", appinfo.ReleaseCycle).
		Str("os", runtime.GOOS).
		Str("arch", runtime.GOARCH).
		Msgf("starting %v shaman-checkout-id-setter", appinfo.ApplicationName)

	log.Warn().Msg("Use with care, and at your own risk.")
	log.Warn().Msg("This is an experimental program, and may ruin your entire Flamenco database.")
	log.Warn().Msg("Press Enter to continue.")
	_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')

	parseCliArgs()

	// Load configuration.
	configService := config.NewService()
	err := configService.Load()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Error().Err(err).Msg("loading configuration")
	}

	// Reject working on a brand new installation.
	isFirstRun, err := configService.IsFirstRun()
	switch {
	case err != nil:
		log.Fatal().Err(err).Msg("unable to determine whether this is the first run of Flamenco or not")
	case isFirstRun:
		log.Fatal().Msg("this should be run on an already-used database")
	}

	// Find the {jobs} variable.
	vars := configService.ResolveVariables(config.VariableAudienceWorkers, config.VariablePlatform(runtime.GOOS))
	jobsPath := vars["jobs"].Value
	if jobsPath == "" {
		log.Fatal().Msg("unable to resolve 'jobs' variable")
	}

	// Connect to the database.
	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer ctxCancel()
	persist := openDB(*configService)
	defer persist.Close()

	// Get all jobs from the database.
	jobs, err := persist.FetchJobs(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to fetch jobs")
	}

	log.Info().Int("numJobs", len(jobs)).Msg("processing all jobs")
	numJobsUpdated := 0
	for _, job := range jobs {
		logger := log.With().Uint("id", job.ID).Str("uuid", job.UUID).Logger()

		if job.Storage.ShamanCheckoutID != "" {
			logger.Info().Str("checkoutID", job.Storage.ShamanCheckoutID).Msg("job already has a Shaman checkout ID")
			continue
		}

		logger.Trace().Msg("processing job")

		// Find the 'blendfile' setting.
		blendfile, ok := job.Settings["blendfile"].(string)
		if !ok {
			logger.Info().Msg("skipping job, it has no `blendfile` setting")
			continue
		}

		// See if it starts with `{jobs}`, otherwise it's not submitted via Shaman.
		relpath, found := strings.CutPrefix(blendfile, "{jobs}"+string(os.PathSeparator))
		if !found {
			logger.Info().Str("blendfile", blendfile).Msg("skipping job, its blendfile setting doesn't start with `{jobs}/`")
			continue
		}

		// See if there is a `pack-info.txt` file next to the blend file. This is
		// another indication that we have the right directory.
		packInfoPath := filepath.Join(jobsPath, filepath.Dir(relpath), "pack-info.txt")
		_, err := os.Stat(packInfoPath)
		switch {
		case errors.Is(err, os.ErrNotExist):
			logger.Warn().Str("packInfo", packInfoPath).Msg("skipping job, pack-info.txt not found where expected")
			continue
		case err != nil:
			logger.Fatal().Str("packInfo", packInfoPath).Msg("error accessing pack-info.txt")
		}

		// Extract the checkout ID from the blend file path.
		checkoutID := filepath.Dir(relpath)
		logger = logger.With().Str("checkoutID", checkoutID).Logger()

		// Store it on the job.
		logger.Debug().Msg("updating job")
		job.Storage.ShamanCheckoutID = checkoutID
		if err := persist.SaveJobStorageInfo(ctx, job); err != nil {
			logger.Error().Err(err).Msg("error saving job to the database")
			continue
		}

		numJobsUpdated++
	}

	log.Info().Msgf("done, updated %d of %d jobs", numJobsUpdated, len(jobs))
}

func parseCliArgs() {
	var quiet, debug, trace bool

	flag.BoolVar(&quiet, "quiet", false, "Only log warning-level and worse.")
	flag.BoolVar(&debug, "debug", false, "Enable debug-level logging.")
	flag.BoolVar(&trace, "trace", false, "Enable trace-level logging.")

	flag.Parse()

	var logLevel zerolog.Level
	switch {
	case trace:
		logLevel = zerolog.TraceLevel
	case debug:
		logLevel = zerolog.DebugLevel
	case quiet:
		logLevel = zerolog.WarnLevel
	default:
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)
}

// openDB opens the database or dies.
func openDB(configService config.Service) *persistence.DB {
	dsn := configService.Get().DatabaseDSN
	if dsn == "" {
		log.Fatal().Msg("configure the database in flamenco-manager.yaml")
	}

	dbCtx, dbCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dbCtxCancel()
	persist, err := persistence.OpenDB(dbCtx, dsn)
	if err != nil {
		log.Fatal().
			Err(err).
			Str("dsn", dsn).
			Msg("error opening database")
	}

	return persist
}
