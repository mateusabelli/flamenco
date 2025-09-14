// Package persistence provides the database interface for Flamenco Manager.
package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"

	"projects.blender.org/studio/flamenco/internal/worker/persistence/sqlc"
)

// DB provides the database interface.
type DB struct {
	sqlDB   *sql.DB
	nowfunc func() time.Time

	// See PeriodicIntegrityCheck().
	consistencyCheckRequests chan struct{}
}

func OpenDB(ctx context.Context, dsn string) (*DB, error) {
	log.Info().Str("dsn", dsn).Msg("opening database")

	db, err := openDB(ctx, dsn)
	if err != nil {
		return nil, err
	}

	// Close the database connection if there was some error. This prevents
	// leaking database connections & should remove any write-ahead-log files.
	closeConnOnReturn := true
	defer func() {
		if !closeConnOnReturn {
			return
		}
		if err := db.Close(); err != nil {
			log.Debug().AnErr("cause", err).Msg("cannot close database connection")
		}
	}()

	if err := db.setBusyTimeout(ctx, 5*time.Second); err != nil {
		return nil, err
	}

	// Perform some maintenance at startup, before trying to migrate the database.
	if !db.performIntegrityCheck(ctx) {
		return nil, ErrIntegrity
	}

	db.vacuum(ctx)

	if err := db.migrate(ctx); err != nil {
		return nil, err
	}
	log.Debug().Msg("database automigration successful")

	// Perform post-migration integrity check, just to be sure.
	if !db.performIntegrityCheck(ctx) {
		return nil, ErrIntegrity
	}

	// Perform another vacuum after database migration, as that may have copied a
	// lot of data and then dropped another lot of data.
	db.vacuum(ctx)

	closeConnOnReturn = false
	return db, nil
}

func openDB(ctx context.Context, dsn string) (*DB, error) {
	// Connect to the database.
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// Close the database connection if there was some error. This prevents
	// leaking database connections & should remove any write-ahead-log files.
	closeConnOnReturn := true
	defer func() {
		if !closeConnOnReturn {
			return
		}
		if err := sqlDB.Close(); err != nil {
			log.Debug().AnErr("cause", err).Msg("cannot close database connection")
		}
	}()

	// Only allow a single database connection, to avoid SQLITE_BUSY errors.
	// It's not certain that this'll improve the situation, but it's worth a try.
	sqlDB.SetMaxIdleConns(1) // Max num of connections in the idle connection pool.
	sqlDB.SetMaxOpenConns(1) // Max num of open connections to the database.

	db := DB{
		sqlDB:   sqlDB,
		nowfunc: func() time.Time { return time.Now().UTC() },

		// Buffer one request, so that even when a consistency check is already
		// running, another can be queued without blocking. Queueing more than one
		// doesn't make sense, though.
		consistencyCheckRequests: make(chan struct{}, 1),
	}

	// Always enable foreign key checks, to make SQLite behave like a real database.
	pragmaCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.pragmaForeignKeys(pragmaCtx, true); err != nil {
		return nil, err
	}

	queries := db.queries()

	// Write-ahead-log journal may improve writing speed.
	log.Trace().Msg("enabling SQLite write-ahead-log journal mode")
	if err := queries.PragmaJournalModeWAL(pragmaCtx); err != nil {
		return nil, fmt.Errorf("enabling SQLite write-ahead-log journal mode: %w", err)
	}
	// Switching from 'full' (default) to 'normal' sync may improve writing speed.
	log.Trace().Msg("enabling SQLite 'normal' synchronisation")
	if err := queries.PragmaSynchronousNormal(pragmaCtx); err != nil {
		return nil, fmt.Errorf("enabling SQLite 'normal' sync mode: %w", err)
	}

	closeConnOnReturn = false
	return &db, nil
}

// vacuum executes the SQL "VACUUM" command, and logs any errors.
func (db *DB) vacuum(ctx context.Context) {
	err := db.queries().Vacuum(ctx)
	if err != nil {
		log.Error().Err(err).Msg("error vacuuming database")
	}
}

// Close closes the connection to the database.
func (db *DB) Close() error {
	return db.sqlDB.Close()
}

// queries returns the SQLC Queries struct, connected to this database.
func (db *DB) queries() *sqlc.Queries {
	loggingWrapper := LoggingDBConn{db.sqlDB}
	return sqlc.New(&loggingWrapper)
}

// now returns 'now' as reported by db.nowfunc.
// It always converts the timestamp to UTC.
func (db *DB) now() time.Time {
	return db.nowfunc()
}

func (db *DB) pragmaForeignKeys(ctx context.Context, enabled bool) error {
	var noun string
	switch enabled {
	case false:
		noun = "disabl"
	case true:
		noun = "enabl"
	}

	log.Trace().Msgf("%sing SQLite foreign key checks", noun)

	queries := db.queries()
	if err := queries.PragmaForeignKeysSet(ctx, enabled); err != nil {
		return fmt.Errorf("%sing foreign keys: %w", noun, err)
	}
	fkEnabled, err := db.areForeignKeysEnabled(ctx)
	if err != nil {
		return err
	}
	if fkEnabled != enabled {
		return fmt.Errorf("SQLite database does not want to %se foreign keys, this may cause data loss", noun)
	}

	return nil
}

func (db *DB) areForeignKeysEnabled(ctx context.Context) (bool, error) {
	log.Trace().Msg("checking whether SQLite foreign key checks are enabled")

	queries := db.queries()
	fkEnabled, err := queries.PragmaForeignKeysGet(ctx)
	if err != nil {
		return false, fmt.Errorf("checking whether the database has foreign key checks are enabled: %w", err)
	}
	return fkEnabled, nil
}
