// SPDX-License-Identifier: GPL-3.0-or-later
package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// errDatabaseBusy is returned by this package when the operation could not be
	// performed due to SQLite being busy.
	errDatabaseBusy = errors.New("database busy")
)

// ErrIsDBBusy returns true when the error is a "database busy" error.
func ErrIsDBBusy(err error) bool {
	return errors.Is(err, errDatabaseBusy) || isDatabaseBusyError(err)
}

// isDatabaseBusyError returns true when the error returned by GORM is a
// SQLITE_BUSY error.
func isDatabaseBusyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "SQLITE_BUSY")
}

// setBusyTimeout sets the SQLite busy_timeout busy handler.
// See https://sqlite.org/pragma.html#pragma_busy_timeout
func (db *DB) setBusyTimeout(ctx context.Context, busyTimeout time.Duration) error {
	queries := db.queriesWithoutTX()
	err := queries.PragmaBusyTimeout(ctx, busyTimeout)
	if err != nil {
		return fmt.Errorf("setting busy_timeout: %w", err)
	}
	return nil
}
