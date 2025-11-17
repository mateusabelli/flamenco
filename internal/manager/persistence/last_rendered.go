package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"database/sql"
	"errors"

	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
)

// LastRendered only has one entry in its database table, to indicate the job
// that was the last to receive a "last rendered image" from a Worker.
// This is used to show the global last-rendered image in the web interface.

// SetLastRendered sets this job as the one with the most recent rendered image.
func (db *DB) SetLastRendered(ctx context.Context, jobUUID string) error {
	now := db.nowNullable()
	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		jobID, err := q.FetchJobIDFromUUID(ctx, jobUUID)
		if err != nil {
			return jobError(err, "finding job with UUID %q", jobUUID)
		}

		return q.SetLastRendered(ctx, sqlc.SetLastRenderedParams{
			CreatedAt: now.Time,
			UpdatedAt: now,
			JobID:     jobID,
		})
	})
}

// GetLastRendered returns the UUID of the job with the most recent rendered image.
func (db *DB) GetLastRenderedJobUUID(ctx context.Context) (string, error) {
	var jobUUID string
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		jobUUID, err = q.GetLastRenderedJobUUID(ctx)
		return
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", jobError(err, "finding job with most rencent render")
	}
	return jobUUID, nil
}
