package persistence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteAutoCheckpoint(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(schedulerTestTimeout)
	defer cancel()

	queries := db.queries()
	interval, err := queries.PragmaAutoCheckpointGet(ctx)
	require.NoError(t, err)

	// The WAL auto-checkpoint interval should be 1000, unless modified by a compile-time flag.
	// See https://sqlite.org/pragma.html#pragma_wal_autocheckpoint
	assert.Equal(t, interval, 1000, "expecting the default auto-checkpoint interval to be 1000")
}
