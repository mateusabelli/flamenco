package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"

	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/worker/persistence/sqlc"
)

// LoggingDBConn wraps a database/sql.DB connection, so that it can be used with
// sqlc and log all the queries.
type LoggingDBConn struct {
	wrappedConn sqlc.DBTX
}

var _ sqlc.DBTX = (*LoggingDBConn)(nil)

func (ldbc *LoggingDBConn) ExecContext(ctx context.Context, sql string, args ...interface{}) (sql.Result, error) {
	log.Trace().Str("sql", sql).Interface("args", args).Msg("database: query Exec")
	return ldbc.wrappedConn.ExecContext(ctx, sql, args...)
}
func (ldbc *LoggingDBConn) PrepareContext(ctx context.Context, sql string) (*sql.Stmt, error) {
	log.Trace().Str("sql", sql).Msg("database: query Prepare")
	return ldbc.wrappedConn.PrepareContext(ctx, sql)
}
func (ldbc *LoggingDBConn) QueryContext(ctx context.Context, sql string, args ...interface{}) (*sql.Rows, error) {
	log.Trace().Str("sql", sql).Interface("args", args).Msg("database: query Query")
	return ldbc.wrappedConn.QueryContext(ctx, sql, args...)
}
func (ldbc *LoggingDBConn) QueryRowContext(ctx context.Context, sql string, args ...interface{}) *sql.Row {
	log.Trace().Str("sql", sql).Interface("args", args).Msg("database: query QueryRow")
	return ldbc.wrappedConn.QueryRowContext(ctx, sql, args...)
}
