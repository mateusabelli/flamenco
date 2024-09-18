// Package persistence provides the database interface for Flamenco Manager.
package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
)

// DB provides the database interface.
type DB struct {
	gormDB *gorm.DB

	// See PeriodicIntegrityCheck().
	consistencyCheckRequests chan struct{}
}

// Model contains the common database fields for most model structs.
// It is a copy of the gorm.Model struct, but without the `DeletedAt` field.
// Soft deletion is not used by Flamenco. If it ever becomes necessary to
// support soft-deletion, see https://gorm.io/docs/delete.html#Soft-Delete
type Model struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
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

	if err := setBusyTimeout(db.gormDB, 5*time.Second); err != nil {
		return nil, err
	}

	// Perfom some maintenance at startup, before trying to migrate the database.
	if !db.performIntegrityCheck(ctx) {
		return nil, ErrIntegrity
	}

	db.vacuum()

	if err := db.migrate(ctx); err != nil {
		return nil, err
	}
	log.Debug().Msg("database automigration succesful")

	// Perfom post-migration integrity check, just to be sure.
	if !db.performIntegrityCheck(ctx) {
		return nil, ErrIntegrity
	}

	// Perform another vacuum after database migration, as that may have copied a
	// lot of data and then dropped another lot of data.
	db.vacuum()

	closeConnOnReturn = false
	return db, nil
}

func openDB(ctx context.Context, dsn string) (*DB, error) {
	globalLogLevel := log.Logger.GetLevel()
	dblogger := NewDBLogger(log.Level(globalLogLevel))

	config := gorm.Config{
		Logger:  dblogger,
		NowFunc: nowFunc,
	}

	return openDBWithConfig(dsn, &config)
}

func openDBWithConfig(dsn string, config *gorm.Config) (*DB, error) {
	dialector := sqlite.Open(dsn)
	gormDB, err := gorm.Open(dialector, config)
	if err != nil {
		return nil, err
	}

	db := DB{
		gormDB: gormDB,

		// Buffer one request, so that even when a consistency check is already
		// running, another can be queued without blocking. Queueing more than one
		// doesn't make sense, though.
		consistencyCheckRequests: make(chan struct{}, 1),
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

	// Use the generic sql.DB interface to set some connection pool options.
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	// Only allow a single database connection, to avoid SQLITE_BUSY errors.
	// It's not certain that this'll improve the situation, but it's worth a try.
	sqlDB.SetMaxIdleConns(1) // Max num of connections in the idle connection pool.
	sqlDB.SetMaxOpenConns(1) // Max num of open connections to the database.

	// Always enable foreign key checks, to make SQLite behave like a real database.
	if err := db.pragmaForeignKeys(true); err != nil {
		return nil, err
	}

	// Write-ahead-log journal may improve writing speed.
	log.Trace().Msg("enabling SQLite write-ahead-log journal mode")
	if tx := gormDB.Exec("PRAGMA journal_mode = WAL"); tx.Error != nil {
		return nil, fmt.Errorf("enabling SQLite write-ahead-log journal mode: %w", tx.Error)
	}
	// Switching from 'full' (default) to 'normal' sync may improve writing speed.
	log.Trace().Msg("enabling SQLite 'normal' synchronisation")
	if tx := gormDB.Exec("PRAGMA synchronous = normal"); tx.Error != nil {
		return nil, fmt.Errorf("enabling SQLite 'normal' sync mode: %w", tx.Error)
	}

	closeConnOnReturn = false
	return &db, nil
}

// nowFunc returns 'now' in UTC, so that GORM-managed times (createdAt,
// deletedAt, updatedAt) are stored in UTC.
func nowFunc() time.Time {
	return time.Now().UTC()
}

// vacuum executes the SQL "VACUUM" command, and logs any errors.
func (db *DB) vacuum() {
	tx := db.gormDB.Exec("vacuum")
	if tx.Error != nil {
		log.Error().Err(tx.Error).Msg("error vacuuming database")
	}
}

// Close closes the connection to the database.
func (db *DB) Close() error {
	sqldb, err := db.gormDB.DB()
	if err != nil {
		return err
	}
	return sqldb.Close()
}

// queries returns the SQLC Queries struct, connected to this database.
// It is intended that all GORM queries will be migrated to use this interface
// instead.
//
// Note that this function does not return an error. Instead it just panics when
// it cannot obtain the low-level GORM database interface. I have no idea when
// this will ever fail, so I'm opting to simplify the use of this function
// instead.
func (db *DB) queries() *sqlc.Queries {
	sqldb, err := db.gormDB.DB()
	if err != nil {
		panic(fmt.Sprintf("could not get low-level database driver: %v", err))
	}

	loggingWrapper := LoggingDBConn{sqldb}
	return sqlc.New(&loggingWrapper)
}

type queriesTX struct {
	queries  *sqlc.Queries
	commit   func() error
	rollback func() error
}

// queries returns the SQLC Queries struct, connected to this database.
// It is intended that all GORM queries will be migrated to use this interface
// instead.
//
// After calling this function, all queries should use this transaction until it
// is closed (either committed or rolled back). Otherwise SQLite will deadlock,
// as it will make any other query wait until this transaction is done.
func (db *DB) queriesWithTX() (*queriesTX, error) {
	sqldb, err := db.gormDB.DB()
	if err != nil {
		panic(fmt.Sprintf("could not get low-level database driver: %v", err))
	}

	tx, err := sqldb.Begin()
	if err != nil {
		return nil, fmt.Errorf("could not begin database transaction: %w", err)
	}

	loggingWrapper := LoggingDBConn{tx}

	qtx := queriesTX{
		queries:  sqlc.New(&loggingWrapper),
		commit:   tx.Commit,
		rollback: tx.Rollback,
	}

	return &qtx, nil
}

// now returns the result of `nowFunc()` wrapped in a sql.NullTime.
func (db *DB) now() sql.NullTime {
	return sql.NullTime{
		Time:  db.gormDB.NowFunc(),
		Valid: true,
	}
}

func (db *DB) pragmaForeignKeys(enabled bool) error {
	var (
		value int
		noun  string
	)
	switch enabled {
	case false:
		value = 0
		noun = "disabl"
	case true:
		value = 1
		noun = "enabl"
	}

	log.Trace().Msgf("%sing SQLite foreign key checks", noun)

	// SQLite doesn't seem to like SQL parameters for `PRAGMA`, so `PRAGMA foreign_keys = ?` doesn't work.
	sql := fmt.Sprintf("PRAGMA foreign_keys = %d", value)

	if tx := db.gormDB.Exec(sql); tx.Error != nil {
		return fmt.Errorf("%sing foreign keys: %w", noun, tx.Error)
	}
	fkEnabled, err := db.areForeignKeysEnabled()
	if err != nil {
		return err
	}
	if fkEnabled != enabled {
		return fmt.Errorf("SQLite database does not want to %se foreign keys, this may cause data loss", noun)
	}

	return nil
}

func (db *DB) areForeignKeysEnabled() (bool, error) {
	log.Trace().Msg("checking whether SQLite foreign key checks are enabled")

	var fkEnabled int
	if tx := db.gormDB.Raw("PRAGMA foreign_keys").Scan(&fkEnabled); tx.Error != nil {
		return false, fmt.Errorf("checking whether the database has foreign key checks are enabled: %w", tx.Error)
	}
	return fkEnabled != 0, nil
}
