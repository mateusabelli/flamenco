package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projects.blender.org/studio/flamenco/internal/worker/persistence/sqlc"
	"projects.blender.org/studio/flamenco/pkg/api"
)

const defaultTimeout = 1 * time.Second

func TestUpstreamBufferQueueEmpty(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(defaultTimeout)
	defer cancel()

	// Queue should be empty at first.
	size, err := db.UpstreamBufferQueueSize(ctx)
	require.NoError(t, err)
	assert.Zero(t, size)

	// Getting the first queued item on an empty queue should work.
	first, err := db.UpstreamBufferFrontItem(ctx)
	require.NoError(t, err)
	require.Nil(t, first)
}

func TestUpstreamBufferQueue(t *testing.T) {
	ctx, cancel, db := persistenceTestFixtures(defaultTimeout)
	defer cancel()

	// Mock the clock so we can compare 'created at' timestamps.
	// The database should order by ID anyway, and not by timestamp.
	fixedNow := db.now()
	db.nowfunc = func() time.Time { return fixedNow }

	// Queue an update.
	taskUUID := "3d1e2419-ca9d-4500-bd4f-1e14b5e82947"
	update1 := api.TaskUpdateJSONRequestBody{
		Activity:   ptr("Tešt active"),
		TaskStatus: ptr(api.TaskStatusActive),
	}
	require.NoError(t, db.UpstreamBufferQueue(ctx, taskUUID, update1))

	// Queue should have grown.
	size, err := db.UpstreamBufferQueueSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, size)

	// Queue another update.
	update2 := api.TaskUpdateJSONRequestBody{
		TaskStatus: ptr(api.TaskStatusCompleted),
	}
	require.NoError(t, db.UpstreamBufferQueue(ctx, taskUUID, update2))

	// Queue should have grown again.
	size, err = db.UpstreamBufferQueueSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, size)

	// First update should be at the front of the queue.
	first, err := db.UpstreamBufferFrontItem(ctx)
	require.NoError(t, err)
	{
		expect := TaskUpdate{
			TaskUpdate: sqlc.TaskUpdate{
				ID:        1,
				CreatedAt: fixedNow,
				TaskID:    taskUUID,
				Payload:   []byte(`{"activity":"Tešt active","taskStatus":"active"}`),
			},
		}
		assert.Equal(t, &expect, first)
	}

	// Deleting should work, and should move the next queued item tot the front of
	// the queue.
	require.NoError(t, db.UpstreamBufferDiscard(ctx, first))
	size, err = db.UpstreamBufferQueueSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, size)
	second, err := db.UpstreamBufferFrontItem(ctx)
	require.NoError(t, err)
	{
		expect := TaskUpdate{
			TaskUpdate: sqlc.TaskUpdate{
				ID:        2,
				CreatedAt: fixedNow,
				TaskID:    taskUUID,
				Payload:   []byte(`{"taskStatus":"completed"}`),
			},
		}
		assert.Equal(t, &expect, second)
	}
}

func ptr[T any](value T) *T {
	return &value
}
