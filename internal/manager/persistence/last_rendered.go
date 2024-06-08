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
type LastRendered struct {
	Model
	JobID uint `gorm:"default:0"`
	Job   *Job `gorm:"foreignkey:JobID;references:ID;constraint:OnDelete:CASCADE"`
}

// SetLastRendered sets this job as the one with the most recent rendered image.
func (db *DB) SetLastRendered(ctx context.Context, j *Job) error {
	queries, err := db.queries()
	if err != nil {
		return err
	}

	now := db.now()
	return queries.SetLastRendered(ctx, sqlc.SetLastRenderedParams{
		CreatedAt: now.Time,
		UpdatedAt: now,
		JobID:     int64(j.ID),
	})
}

// GetLastRendered returns the UUID of the job with the most recent rendered image.
func (db *DB) GetLastRenderedJobUUID(ctx context.Context) (string, error) {
	queries, err := db.queries()
	if err != nil {
		return "", err
	}

	jobUUID, err := queries.GetLastRenderedJobUUID(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", jobError(err, "finding job with most rencent render")
	}
	return jobUUID, nil
}
