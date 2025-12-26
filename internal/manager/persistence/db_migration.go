package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"math"
	"strings"

	goose "github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/pkg/website"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// Directory inside 'embedMigrations'.
const migrationsDir = "migrations"

// Migrate the database, returning true if anything happened, false otherwise.
func (db *DB) migrate(ctx context.Context) (bool, error) {
	// Set up Goose.
	gooseLogger := GooseLogger{}
	goose.SetLogger(&gooseLogger)
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		log.Fatal().AnErr("cause", err).Msg("could not tell Goose to use sqlite3")
	}

	// See if migration is necessary to begin with. If not, we can skip on the
	// disabling & re-enabling of the foreign key constraints, keeping the DB safe.
	currentVersion, err := goose.EnsureDBVersionContext(ctx, db.sqlDB)
	if err != nil {
		return false, fmt.Errorf("setting up database for migrations: %w", err)
	}
	_, err = goose.CollectMigrations(migrationsDir, currentVersion, math.MaxInt64)
	switch {
	case errors.Is(err, goose.ErrNoMigrationFiles):
		log.Debug().Int64("version", currentVersion).Msg("database: at latest version, no migration necessary")
		return false, nil
	case err != nil:
		return false, fmt.Errorf("collecting database migrations: %w", err)
	}

	// Disable foreign key constraints during the migrations. This is necessary
	// for SQLite to do column renames / drops, as that requires creating a new
	// table with the new schema, copying the data, dropping the old table, and
	// moving the new one in its place. That table drop shouldn't trigger 'ON
	// DELETE' actions on foreign keys.
	//
	// Since migration is 99% schema changes, and very little to no manipulation
	// of data, foreign keys are disabled here instead of in the migration SQL
	// files, so that it can't be forgotten.

	if err := db.pragmaForeignKeys(ctx, false); err != nil {
		log.Fatal().AnErr("cause", err).Msgf("could not disable foreign key constraints before performing database migrations, please report a bug at %s", website.BugReportURL)
	}

	// Run Goose.
	log.Debug().Msg("migrating database with Goose")
	if err := goose.UpContext(ctx, db.sqlDB, migrationsDir); err != nil {
		log.Fatal().AnErr("cause", err).Msg("could not migrate database to the latest version")
	}

	// Re-enable foreign key checks.
	if err := db.pragmaForeignKeys(ctx, true); err != nil {
		log.Fatal().AnErr("cause", err).Msgf("could not re-enable foreign key constraints after performing database migrations, please report a bug at %s", website.BugReportURL)
	}

	return true, nil
}

type GooseLogger struct{}

func (gl *GooseLogger) Fatalf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	log.Fatal().Msg(strings.TrimSpace(msg))
}

func (gl *GooseLogger) Printf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	log.Debug().Msg(strings.TrimSpace(msg))
}
