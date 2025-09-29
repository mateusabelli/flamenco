package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/pkg/website"
)

var ErrIntegrity = errors.New("database integrity check failed")

const (
	integrityCheckTimeout = 10 * time.Second

	// How often the database write-ahead log is checkpointed.
	walCheckpointPeriod = 15 * time.Minute
)

// PeriodicIntegrityCheck periodically checks the database integrity.
// This function only returns when the context is done.
func (db *DB) PeriodicIntegrityCheck(
	ctx context.Context,
	period time.Duration,
	onErrorCallback func(),
) {
	if period == 0 {
		log.Info().Msg("database: periodic integrity check disabled")
		return
	}

	log.Info().
		Stringer("period", period).
		Msg("database: periodic integrity check starting")
	defer log.Debug().Msg("database: periodic integrity check stopping")

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(period):
		case <-db.consistencyCheckRequests:
		}

		ok := db.performIntegrityCheck(ctx)
		if !ok {
			log.Error().Msg("database: periodic integrity check failed")
			onErrorCallback()
		}
	}
}

// RequestIntegrityCheck triggers a check of the database persistency.
func (db *DB) RequestIntegrityCheck() {
	select {
	case db.consistencyCheckRequests <- struct{}{}:
		// Don't do anything, the work is done.
	default:
		log.Debug().Msg("database: could not trigger integrity check, another check might already be queued.")
	}
}

// performIntegrityCheck uses a few 'pragma' SQL statements to do some integrity checking.
// Returns true on OK, false if there was an issue. Issues are always logged.
func (db *DB) performIntegrityCheck(ctx context.Context) (ok bool) {
	checkCtx, cancel := context.WithTimeout(ctx, integrityCheckTimeout)
	defer cancel()

	log.Debug().Msg("database: performing integrity check")

	db.ensureForeignKeysEnabled(checkCtx)

	if !db.pragmaIntegrityCheck(checkCtx) {
		return false
	}
	return db.pragmaForeignKeyCheck(checkCtx)
}

// pragmaIntegrityCheck checks database file integrity. This does not include
// foreign key checks.
//
// Returns true on OK, false if there was an issue. Issues are always logged.
//
// See https: //www.sqlite.org/pragma.html#pragma_integrity_check
func (db *DB) pragmaIntegrityCheck(ctx context.Context) (ok bool) {
	queries := db.queries()
	issues, err := queries.PragmaIntegrityCheck(ctx)
	if err != nil {
		log.Error().Err(err).Msg("database: error checking integrity")
		return false
	}

	switch len(issues) {
	case 0:
		log.Warn().Msg("database: integrity check returned nothing, expected explicit 'ok'; treating as an implicit 'ok'")
		return true
	case 1:
		if issues[0].Description == "ok" {
			log.Debug().Msg("database: integrity check ok")
			return true
		}
	}

	log.Error().Int("num_issues", len(issues)).Msg("database: integrity check failed")
	for _, issue := range issues {
		log.Error().
			Str("description", issue.Description).
			Msg("database: integrity check failure")
	}

	return false
}

// pragmaForeignKeyCheck checks whether all foreign key constraints are still valid.
//
// SQLite has optional foreign key relations, so even though Flamenco Manager
// always enables these on startup, at some point there could be some issue
// causing these checks to be skipped.
//
// Returns true on OK, false if there was an issue. Issues are always logged.
//
// See https: //www.sqlite.org/pragma.html#pragma_foreign_key_check
func (db *DB) pragmaForeignKeyCheck(ctx context.Context) (ok bool) {
	queries := db.queries()

	issues, err := queries.PragmaForeignKeyCheck(ctx)
	if err != nil {
		log.Error().Err(err).Msg("database: error checking foreign keys")
		return false
	}

	if len(issues) == 0 {
		log.Debug().Msg("database: foreign key check ok")
		return true
	}

	log.Error().Int("num_issues", len(issues)).Msg("database: foreign key check failed")
	for _, issue := range issues {
		log.Error().
			Str("table", issue.Table).
			Int("rowid", issue.RowID).
			Str("parent", issue.Parent).
			Int("fkid", issue.FKID).
			Msg("database: foreign key relation missing")
	}

	return false
}

// ensureForeignKeysEnabled checks whether foreign keys are enabled, and if not,
// tries to enable them.
func (db *DB) ensureForeignKeysEnabled(ctx context.Context) {
	fkEnabled, err := db.areForeignKeysEnabled(ctx)

	if err != nil {
		log.Error().AnErr("cause", err).Msg("database: could not check whether foreign keys are enabled")
		return
	}

	if fkEnabled {
		return
	}

	log.Warn().Msg("database: foreign keys are disabled, re-enabling them")
	if err := db.pragmaForeignKeys(ctx, true); err != nil {
		log.Error().AnErr("cause", err).Msg("database: error re-enabling foreign keys")
		return
	}
}

func (db *DB) PeriodicWALCheckpoint(ctx context.Context) {

	if err := db.walCheckpoint(ctx); err != nil {
		log.Error().
			AnErr("cause", err).
			Msgf("database: could not perform checkpointing operation on write-ahead log at startup. Please report a bug at %s", website.BugReportURL)
		// Still keep going, to enable the periodic checkpointing.
	}

	log.Info().
		Stringer("period", walCheckpointPeriod).
		Msg("database: will perform periodic checkpoint")

	defer log.Info().Msg("database: stopped periodic checkpoint")

	ticker := time.NewTicker(walCheckpointPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			err := db.walCheckpoint(ctx)

			switch {
			case err == nil:
				// Yay
			case errors.Is(err, context.Canceled) && ctx.Err() != nil:
				// Main context got cancelled, which means a shutdown. That's fine, so
				// just log a debug message that this happened during a checkpoint.
				log.Debug().Msg("database: application is shutting down during a checkpoint operation")
				// Just continue and let the normal `<-ctx.Done()` case above handle the
				// main context closing. That way there's one point where that's done.
			default:
				log.Error().
					AnErr("cause", err).
					Msgf("database: could not perform checkpointing operation on write-ahead log. Please report a bug at %s", website.BugReportURL)
			}
		}
	}
}

// walCheckpoint performs a checkpoint of the write-ahead log (WAL).
// See https://sqlite.org/wal.html and https://sqlite.org/pragma.html#pragma_wal_checkpoint
func (db *DB) walCheckpoint(ctx context.Context) error {
	qtx, err := db.queriesWithTX()
	defer qtx.rollback()
	if err != nil {
		return fmt.Errorf("starting database transaction: %w", err)
	}

	result, err := qtx.queries.WALCheckpointFull(ctx)
	if err != nil {
		return err
	}
	if err := qtx.commit(); err != nil {
		return fmt.Errorf("committing database transaction: %w", err)
	}

	// Number of pages that can be in the WAL log before operations show up at
	// INFO level.
	//
	// Having pages in the WAL is expected (it's what it's for), and by default
	// sqlite should auto-checkpoint at 1000 pages. However, at Blender Studio
	// there was an issue where this did not happen, or at least did not kick in
	// before the WAL file became >10 GB. That shouldn't happen now that Flamenco
	// is doing periodic checkpointing, but it's still nice to be able to see any
	// gradual increase before that 1000 pages is hit.
	//
	// Maybe this threshold should be increased at some point, if it turns out
	// that the logging is confusing users.
	const threshold = 250

	// The log level is determined by what happened.
	var logLevel zerolog.Level
	switch {
	case result.Busy > 0:
		logLevel = zerolog.WarnLevel
	case result.Log > threshold || result.Checkpointed > threshold:
		logLevel = zerolog.InfoLevel
	default:
		logLevel = zerolog.DebugLevel
	}

	log.WithLevel(logLevel).
		Bool("busy", result.Busy > 0).
		Int64("pagesInWAL", result.Log).
		Int64("checkpointedPages", result.Checkpointed).
		Msg("database: checkpoint complete")

	return nil
}
