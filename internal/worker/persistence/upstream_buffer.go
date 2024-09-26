package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"projects.blender.org/studio/flamenco/internal/worker/persistence/sqlc"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// SPDX-License-Identifier: GPL-3.0-or-later

// TaskUpdate is a queued task update.
type TaskUpdate struct {
	sqlc.TaskUpdate
}

func (t *TaskUpdate) Unmarshal() (*api.TaskUpdateJSONRequestBody, error) {
	var apiTaskUpdate api.TaskUpdateJSONRequestBody
	if err := json.Unmarshal(t.Payload, &apiTaskUpdate); err != nil {
		return nil, err
	}
	return &apiTaskUpdate, nil
}

// UpstreamBufferQueueSize returns how many task updates are queued in the upstream buffer.
func (db *DB) UpstreamBufferQueueSize(ctx context.Context) (int, error) {
	queries := db.queries()

	queueSize, err := queries.CountTaskUpdates(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting queued task updates: %w", err)
	}
	return int(queueSize), nil
}

// UpstreamBufferQueue queues a task update in the upstrema buffer.
func (db *DB) UpstreamBufferQueue(ctx context.Context, taskID string, apiTaskUpdate api.TaskUpdateJSONRequestBody) error {
	blob, err := json.Marshal(apiTaskUpdate)
	if err != nil {
		return fmt.Errorf("converting task update to JSON: %w", err)
	}

	queries := db.queries()
	err = queries.InsertTaskUpdate(ctx, sqlc.InsertTaskUpdateParams{
		CreatedAt: db.now(),
		TaskID:    taskID,
		Payload:   blob,
	})

	return err
}

// UpstreamBufferFrontItem returns the first-queued item. The item remains queued.
func (db *DB) UpstreamBufferFrontItem(ctx context.Context) (*TaskUpdate, error) {
	queries := db.queries()
	result, err := queries.FirstTaskUpdate(ctx)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	}
	return &TaskUpdate{result}, err
}

// UpstreamBufferDiscard discards the queued task update with the given row ID.
func (db *DB) UpstreamBufferDiscard(ctx context.Context, queuedTaskUpdate *TaskUpdate) error {
	queries := db.queries()
	return queries.DeleteTaskUpdate(ctx, queuedTaskUpdate.ID)
}
