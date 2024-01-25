package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"embed"
	"fmt"
	"strings"

	goose "github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/pkg/website"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func (db *DB) migrate(ctx context.Context) error {
	// Set up Goose.
	gooseLogger := GooseLogger{}
	goose.SetLogger(&gooseLogger)
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		log.Fatal().AnErr("cause", err).Msg("could not tell Goose to use sqlite3")
	}

	// Hook up Goose to the database.
	lowLevelDB, err := db.gormDB.DB()
	if err != nil {
		log.Fatal().AnErr("cause", err).Msg("GORM would not give us its low-level interface")
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

	if err := db.pragmaForeignKeys(false); err != nil {
		log.Fatal().AnErr("cause", err).Msgf("could not disable foreign key constraints before performing database migrations, please report a bug at %s", website.BugReportURL)
	}

	// Run Goose.
	log.Debug().Msg("migrating database with Goose")
	if err := goose.UpContext(ctx, lowLevelDB, "migrations"); err != nil {
		log.Fatal().AnErr("cause", err).Msg("could not migrate database to the latest version")
	}

	// Re-enable foreign key checks.
	if err := db.pragmaForeignKeys(true); err != nil {
		log.Fatal().AnErr("cause", err).Msgf("could not re-enable foreign key constraints after performing database migrations, please report a bug at %s", website.BugReportURL)
	}

	return nil
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
