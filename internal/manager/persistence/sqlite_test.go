package persistence

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sqliteTestTimeout = 1 * time.Second

// TestSQLiteAutoCheckpoint checks the WAL auto-checkpoint interval. It is here
// just to ensure the SQLite implementation adheres to our expectations.
func TestSQLiteAutoCheckpoint(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(sqliteTestTimeout)
	defer cancel()

	queries := db.queriesWithoutTX()
	interval, err := queries.PragmaAutoCheckpointGet(ctx)
	require.NoError(t, err)

	// The WAL auto-checkpoint interval should be 1000, unless modified by a compile-time flag.
	// See https://sqlite.org/pragma.html#pragma_wal_autocheckpoint
	assert.Equal(t, interval, 1000, "expecting the default auto-checkpoint interval to be 1000")
}

func TestDefaultPragmaValues(t *testing.T) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), sqliteTestTimeout)
	defer ctxCancel()

	// This tests an actual SQLite file connection, and not the in-memory database
	// used by other unit tests. The journalling options for in-memory databases
	// are different.
	db, err := openDB(ctx, "sqlite_test.sqlite")
	require.NoError(t, err)
	defer func() {
		db.Close()
		os.Remove("sqlite_test.sqlite")
	}()

	// Get low-level connections to test, instead of using the SQLC-generated
	// code. This is to check that newly-created database connections have the
	// correct settings.
	//
	// This is also why multiple connections are made in parallel, to ensure that
	// this all works as expected.
	connections := make([]*sql.Conn, 5)
	for index := range connections {
		conn, err := db.sqlDB.Conn(ctx)
		require.NoError(t, err)
		connections[index] = conn
	}

	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
	}()

	for _, conn := range connections {
		{
			row := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`)
			var fkEnabled bool
			err := row.Scan(&fkEnabled)
			require.NoError(t, err)
			require.True(t, fkEnabled, "foreign key constraints should be enabled after connecting")
		}

		{
			row := conn.QueryRowContext(ctx, `PRAGMA journal_mode`)
			var journalMode string
			err := row.Scan(&journalMode)
			require.NoError(t, err)
			require.Equal(t, "wal", journalMode, "'journal_mode' should be 'wal' after connecting")
		}

		{
			row := conn.QueryRowContext(ctx, `PRAGMA synchronous`)
			var synchronous int
			err := row.Scan(&synchronous)
			require.NoError(t, err)
			require.Equal(t, 1, synchronous, "'synchronous' should be 'normal(1)' after connecting")
		}
	}

}
