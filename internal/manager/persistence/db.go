// Package persistence provides the database interface for Flamenco Manager.
package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/powerman/fileuri"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"

	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
)

const (
	// busyTimeout is the global timeout for database queries.
	busyTimeout = 20 * time.Second

	// connMaxIdleDuration sets the maximum length of time a connection can be idle before it is closed.
	connMaxIdleDuration = 10 * time.Minute
	// connMaxLifeDuration sets the maximum length of time a connection can be held open before it is closed.
	connMaxLifeDuration = 1 * time.Hour
)

// DB provides the database interface.
type DB struct {
	sqlDB   *sql.DB
	nowfunc func() time.Time

	// See PeriodicIntegrityCheck().
	consistencyCheckRequests chan struct{}

	mutex *sync.RWMutex
}

// Model contains the common database fields for most model structs.
// It is a copy of the gorm.Model struct, but without the `DeletedAt` field.
// Soft deletion is not used by Flamenco. If it ever becomes necessary to
// support soft-deletion, see https://gorm.io/docs/delete.html#Soft-Delete
type Model struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time
}

func OpenDB(ctx context.Context, sqliteFile string) (*DB, error) {
	log.Info().Str("file", sqliteFile).Msg("opening database")

	// 'sqliteFile' should just be a file path to a sqlite file. If its directory
	// doesn't exist yet, create it. Otherwise sqlite will come back with a
	// cryptic error message "unable to open database file: out of memory".
	dbDirectory := filepath.Dir(sqliteFile)
	if err := os.MkdirAll(dbDirectory, os.ModePerm); err != nil {
		return nil, fmt.Errorf("creating database directory %s: %w", dbDirectory, err)
	}

	db, err := openDB(ctx, sqliteFile)
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

	if err := db.setBusyTimeout(ctx, busyTimeout); err != nil {
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

func openDB(ctx context.Context, sqliteFile string) (*DB, error) {
	var (
		dsnURL *url.URL
		err    error
	)

	// Before converting a file path to a URL, it has to be made absolute. This
	// shouldn't happen to `file::memory:` "files" though.
	if !strings.HasPrefix(sqliteFile, "file:") {
		sqliteFile, err = filepath.Abs(sqliteFile)
		if err != nil {
			return nil, err
		}
		dsnURL, err = fileuri.FromFilePath(sqliteFile)
	} else {
		dsnURL, err = url.Parse(sqliteFile)
	}
	if err != nil {
		return nil, fmt.Errorf("converting path to sqlite (%q) to a URL: %w", sqliteFile, err)
	}

	// Connect to the database, setting various PRAGMAs via the DSN. This ensures
	// that re-connections made by the sql package always start out with foreign
	// keys enabled, the right journal mode, etc.
	query := dsnURL.Query()
	query.Add("_pragma", "foreign_keys = 1")
	query.Add("_pragma", "journal_mode = WAL")
	query.Add("_pragma", "synchronous = normal")
	dsnURL.RawQuery = query.Encode()

	log.Debug().Stringer("dsnURL", dsnURL).Msg("database: opening database URL")
	sqlDB, err := sql.Open("sqlite", dsnURL.String())
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

	sqlDB.SetMaxIdleConns(5)  // Max num of connections in the idle connection pool.
	sqlDB.SetMaxOpenConns(10) // Max num of open connections to the database.
	sqlDB.SetConnMaxIdleTime(connMaxIdleDuration)
	sqlDB.SetConnMaxLifetime(connMaxLifeDuration)

	db := DB{
		sqlDB:   sqlDB,
		nowfunc: func() time.Time { return time.Now().UTC() },

		// Buffer one request, so that even when a consistency check is already
		// running, another can be queued without blocking. Queueing more than one
		// doesn't make sense, though.
		consistencyCheckRequests: make(chan struct{}, 1),

		mutex: new(sync.RWMutex),
	}

	//
	pragmaCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if fkEnabled, err := db.areForeignKeysEnabled(pragmaCtx); err != nil {
		return nil, err
	} else if !fkEnabled {
		return nil, errors.New("foreign keys are disabled, refusing to start")
	}

	closeConnOnReturn = false
	return &db, nil
}

// vacuum executes the SQL "VACUUM" command, and logs any errors.
func (db *DB) vacuum(ctx context.Context) {
	err := db.queriesWithoutTX().Vacuum(ctx)
	if err != nil {
		log.Error().Err(err).Msg("error vacuuming database")
	}
}

// Close closes the connection to the database.
func (db *DB) Close() error {
	return db.sqlDB.Close()
}

// queriesWithoutTX returns the SQLC Queries struct, connected to this database.
//
// This does NOT run in any transaction. It is preferred to use db.queriesRW()
// or db.queriesRO().
func (db *DB) queriesWithoutTX() *sqlc.Queries {
	loggingWrapper := LoggingDBConn{db.sqlDB}
	return sqlc.New(&loggingWrapper)
}

// queriesRW creates a read/write transaction.
func (db *DB) queriesRW(
	ctx context.Context,
	callback func(*sqlc.Queries) error,
) error {
	// Only allow a single writing transaction at a time. Doing this on the Go
	// side makes things much simpler than doing it via SQLite (which would
	// require responding to SQLITE_BUSY errors and re-trying queries until they
	// succeed).
	//
	// Also see
	// https://berthub.eu/articles/posts/a-brief-post-on-sqlite3-database-locked-despite-timeout/:
	// it describes that this can also be solved by starting transactions with
	// `BEGIN IMMEDIATE` when you know they'll have to write. However, with the
	// database/sql it's hard to do this on a per-transaction basis. And then
	// still you can get SQLITE_BUSY errors. Hence the lock on the Go side.
	db.mutex.Lock()
	defer db.mutex.Unlock()

	queriesTX, err := db._queriesCtx(ctx, false)
	if err != nil {
		return err
	}

	if err = callback(queriesTX.queries); err != nil {
		queriesTX.rollback()
		return err
	}

	return queriesTX.commit()
}

// queriesRO creates a read-only transaction.
//
// NOTE: if the query accidentally does write to the database it will NOT cause
// any error. This seems to be a limitation of the SQLite driver. To prevent
// such queries from modifying the database, the transaction is always rolled
// back.
func (db *DB) queriesRO(
	ctx context.Context,
	callback func(*sqlc.Queries) error,
) error {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	queriesTX, err := db._queriesCtx(ctx, true)
	if err != nil {
		return err
	}

	// Read-only transactions are always rolled back, as there is nothing to commit.
	defer queriesTX.rollback()

	return callback(queriesTX.queries)
}

func (db *DB) _queriesCtx(ctx context.Context, readOnly bool) (*queriesTX, error) {
	tx, err := db.sqlDB.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
		ReadOnly:  readOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("could not begin database transaction: %w", err)
	}

	loggingWrapper := LoggingDBConn{tx}

	qtx := queriesTX{
		queries:  sqlc.New(&loggingWrapper),
		commit:   commitWrapper(tx.Commit),
		rollback: rollbackWrapper(tx.Rollback),
	}

	return &qtx, nil
}

type queriesTX struct {
	queries  *sqlc.Queries
	commit   func() error
	rollback func()
}

// commitWrapper wraps any error returned by `commit()` to make it explicit that
// it's from a commit function.
func commitWrapper(commit func() error) func() error {
	return func() error {
		err := commit()
		if err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		return nil
	}
}

func rollbackWrapper(rollback func() error) func() {
	return func() {
		err := rollback()

		// AThis function is typically called unconditionally via `defer` and so the
		// most common case is that the transaction has already been committed, and
		// thus ErrTxDone is returned here.

		switch {
		case err == nil: // Not really expected, but a good rollback is ok.
		case errors.Is(err, sql.ErrTxDone): // Expected.
		default:
			log.Error().Msg("database: query rollback failed unexpectedly")
		}
	}
}

// now returns 'now' as reported by db.nowfunc.
// It always converts the timestamp to UTC.
func (db *DB) now() time.Time {
	return db.nowfunc()
}

// nowNullable returns the result of `now()` wrapped in a sql.NullTime.
// It is nullable just for ease of use, it will never actually be null.
func (db *DB) nowNullable() sql.NullTime {
	return sql.NullTime{
		Time:  db.now(),
		Valid: true,
	}
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

	queries := db.queriesWithoutTX()
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

	queries := db.queriesWithoutTX()
	fkEnabled, err := queries.PragmaForeignKeysGet(ctx)
	if err != nil {
		return false, fmt.Errorf("checking whether the database has foreign key checks are enabled: %w", err)
	}
	return fkEnabled, nil
}
