package main

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"

	_ "modernc.org/sqlite"
)

var (
	// Tables and/or indices to skip when writing the schema.
	// Anything that is *not* to be seen by sqlc should be listed here.
	skips = map[SQLiteSchema]bool{
		// Goose manages its own versioning table. SQLC should ignore its existence.
		{Type: "table", Name: "goose_db_version"}: true,
	}

	tableNameDequoter = regexp.MustCompile("^(?:CREATE TABLE )(\"([^\"]+)\")")
)

type SQLiteSchema struct {
	Type      string
	Name      string
	TableName string
	RootPage  int
	SQL       sql.NullString
}

func saveSchema(ctx context.Context, sqlOutPath string) error {
	db, err := sql.Open("sqlite", "flamenco-manager.sqlite")
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "select * from sqlite_schema order by type desc, name asc")
	if err != nil {
		return err
	}
	defer rows.Close()

	sqlBuilder := strings.Builder{}

	for rows.Next() {
		var data SQLiteSchema
		if err := rows.Scan(
			&data.Type,
			&data.Name,
			&data.TableName,
			&data.RootPage,
			&data.SQL,
		); err != nil {
			return err
		}
		if strings.HasPrefix(data.Name, "sqlite_") {
			continue
		}
		if skips[SQLiteSchema{Type: data.Type, Name: data.Name}] {
			continue
		}
		if !data.SQL.Valid {
			continue
		}

		sql := tableNameDequoter.ReplaceAllString(data.SQL.String, "CREATE TABLE $2")

		sqlBuilder.WriteString(sql)
		sqlBuilder.WriteString(";\n")
	}

	sqlBytes := []byte(sqlBuilder.String())
	if err := os.WriteFile(sqlOutPath, sqlBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing to %s: %w", sqlOutPath, err)
	}

	log.Info().Str("path", sqlOutPath).Msg("schema written to file")
	return nil
}

// SqlcConfig models the minimal subset of the sqlc.yaml we need to parse.
type SqlcConfig struct {
	Version string `yaml:"version"`
	SQL     []struct {
		Schema string `yaml:"schema"`
	} `yaml:"sql"`
}

func main() {
	output := zerolog.ConsoleWriter{Out: colorable.NewColorableStdout(), TimeFormat: time.RFC3339}
	log.Logger = log.Output(output)
	parseCliArgs()

	mainCtx, mainCtxCancel := context.WithCancel(context.Background())
	defer mainCtxCancel()

	installSignalHandler(mainCtxCancel)

	schemaPath := schemaPathFromSqlcYAML()

	if err := saveSchema(mainCtx, schemaPath); err != nil {
		log.Fatal().Err(err).Msg("couldn't export schema")
	}
}

// installSignalHandler spawns a goroutine that handles incoming POSIX signals.
func installSignalHandler(cancelFunc context.CancelFunc) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	signal.Notify(signals, syscall.SIGTERM)
	go func() {
		for signum := range signals {
			log.Info().Str("signal", signum.String()).Msg("signal received, shutting down")
			cancelFunc()
		}
	}()
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

func schemaPathFromSqlcYAML() string {
	var sqlcConfig SqlcConfig

	{
		sqlcConfigBytes, err := os.ReadFile("sqlc.yaml")
		if err != nil {
			log.Fatal().Err(err).Msg("cannot read sqlc.yaml")
		}

		if err := yaml.Unmarshal(sqlcConfigBytes, &sqlcConfig); err != nil {
			log.Fatal().Err(err).Msg("cannot parse sqlc.yaml")
		}
	}

	if sqlcConfig.Version != "2" {
		log.Fatal().
			Str("version", sqlcConfig.Version).
			Str("expected", "2").
			Msg("unexpected version in sqlc.yaml")
	}

	if len(sqlcConfig.SQL) == 0 {
		log.Fatal().
			Int("sql items", len(sqlcConfig.SQL)).
			Msg("sqlc.yaml should contain at least one item in the 'sql' list")
	}

	schema := sqlcConfig.SQL[0].Schema
	if schema == "" {
		log.Fatal().Msg("sqlc.yaml should have a 'schema' key in the first 'sql' item")
	}

	return schema
}
