package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
)

// dbLogger implements the behaviour of Gorm's default logger on top of Zerolog.
// See https://github.com/go-gorm/gorm/blob/master/logger/logger.go
type dbLogger struct {
	zlog *zerolog.Logger

	IgnoreRecordNotFoundError bool
	SlowThreshold             time.Duration
}

var _ gormlogger.Interface = (*dbLogger)(nil)

var logLevelMap = map[gormlogger.LogLevel]zerolog.Level{
	gormlogger.Silent: zerolog.Disabled,
	gormlogger.Error:  zerolog.ErrorLevel,
	gormlogger.Warn:   zerolog.WarnLevel,
	gormlogger.Info:   zerolog.InfoLevel,
}

func gormToZlogLevel(logLevel gormlogger.LogLevel) zerolog.Level {
	zlogLevel, ok := logLevelMap[logLevel]
	if !ok {
		// Just a default value that seemed sensible at the time of writing.
		return zerolog.DebugLevel
	}
	return zlogLevel
}

// NewDBLogger wraps a zerolog logger to implement a Gorm logger interface.
func NewDBLogger(zlog zerolog.Logger) *dbLogger {
	return &dbLogger{
		zlog: &zlog,
		// Remaining properties default to their zero value.
	}
}

// LogMode returns a child logger at the given log level.
func (l *dbLogger) LogMode(logLevel gormlogger.LogLevel) gormlogger.Interface {
	childLogger := l.zlog.Level(gormToZlogLevel(logLevel))
	newlogger := *l
	newlogger.zlog = &childLogger
	return &newlogger
}

func (l *dbLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	l.logEvent(zerolog.InfoLevel, msg, args)
}

func (l *dbLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	l.logEvent(zerolog.WarnLevel, msg, args)
}

func (l *dbLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	l.logEvent(zerolog.ErrorLevel, msg, args)
}

// Trace traces the execution of SQL and potentially logs errors, warnings, and infos.
// Note that it doesn't mean "trace-level logging".
func (l *dbLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	zlogLevel := l.zlog.GetLevel()
	if zlogLevel == zerolog.Disabled {
		return
	}

	elapsed := time.Since(begin)
	logCtx := l.zlog.With().CallerWithSkipFrameCount(5)

	// Function to lazily get the SQL, affected rows, and logger.
	buildLogger := func() (loggerPtr *zerolog.Logger, sql string) {
		sql, rows := fc()
		logCtx = logCtx.AnErr("cause", err)
		if rows >= 0 {
			logCtx = logCtx.Int64("rowsAffected", rows)
		}
		logger := logCtx.Logger()
		return &logger, sql
	}

	switch {
	case err != nil && zlogLevel <= zerolog.ErrorLevel:
		logger, sql := buildLogger()
		if l.silenceLoggingError(err) {
			logger.Debug().Msg(sql)
		} else {
			logger.Error().Msg(sql)
		}

	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && zlogLevel <= zerolog.WarnLevel:
		logger, sql := buildLogger()
		logger.Warn().
			Str("sql", sql).
			Dur("elapsed", elapsed).
			Dur("slowThreshold", l.SlowThreshold).
			Msg("slow database query")

	case zlogLevel <= zerolog.TraceLevel:
		logger, sql := buildLogger()
		logger.Trace().Msg(sql)
	}
}

func (l dbLogger) silenceLoggingError(err error) bool {
	switch {
	case l.IgnoreRecordNotFoundError && errors.Is(err, gorm.ErrRecordNotFound):
		return true
	case errors.Is(err, context.Canceled):
		// These are usually caused by the HTTP client connection closing. Stopping
		// a database query is normal behaviour in such a case, so this shouldn't be
		// logged as an error.
		return true
	default:
		return false
	}
}

// logEvent logs an even at the given level.
func (l dbLogger) logEvent(level zerolog.Level, msg string, args ...interface{}) {
	if l.zlog.GetLevel() > level {
		return
	}
	logger := l.logger(args)
	logger.WithLevel(level).Msg("logEvent: " + msg)
}

// logger constructs a zerolog logger. The given arguments are added via reflection.
func (l dbLogger) logger(args ...interface{}) zerolog.Logger {
	logCtx := l.zlog.With()
	for idx, arg := range args {
		logCtx.Interface(fmt.Sprintf("arg%d", idx), arg)
	}
	return logCtx.Logger()
}

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
