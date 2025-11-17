package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"fmt"

	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
)

type WorkerTag = sqlc.WorkerTag

func (db *DB) CreateWorkerTag(ctx context.Context, tag *WorkerTag) error {
	now := db.now()
	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		dbID, err := q.CreateWorkerTag(ctx, sqlc.CreateWorkerTagParams{
			CreatedAt:   now,
			UUID:        tag.UUID,
			Name:        tag.Name,
			Description: tag.Description,
		})
		if err != nil {
			return fmt.Errorf("creating new worker tag: %w", err)
		}

		tag.ID = dbID
		tag.CreatedAt = now

		return nil
	})
}

// HasWorkerTags returns whether there are any tags defined at all.
func (db *DB) HasWorkerTags(ctx context.Context) (bool, error) {
	var count int64
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		count, err = q.CountWorkerTags(ctx)
		return
	})
	return count > 0, workerTagError(err, "counting worker tags")
}

func (db *DB) FetchWorkerTag(ctx context.Context, uuid string) (WorkerTag, error) {
	var workerTag WorkerTag
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		workerTag, err = q.FetchWorkerTagByUUID(ctx, uuid)
		return
	})

	return workerTag, workerTagError(err, "fetching worker tag")
}

func (db *DB) FetchWorkerTagByName(ctx context.Context, name string) (WorkerTag, error) {
	var workerTag WorkerTag
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		workerTag, err = q.FetchWorkerTagByName(ctx, name)
		return
	})

	return workerTag, workerTagError(err, "fetching worker tag")
}

func (db *DB) FetchWorkerTagByID(ctx context.Context, id int64) (WorkerTag, error) {
	var workerTag WorkerTag
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		workerTag, err = q.FetchWorkerTagByID(ctx, id)
		return
	})
	return workerTag, workerTagError(err, "fetching worker tag")
}

func (db *DB) SaveWorkerTag(ctx context.Context, tag *WorkerTag) error {
	params := sqlc.SaveWorkerTagParams{
		UpdatedAt:   db.nowNullable(),
		UUID:        tag.UUID,
		Name:        tag.Name,
		Description: tag.Description,
		WorkerTagID: tag.ID,
	}
	err := db.queriesRW(ctx, func(q *sqlc.Queries) error {
		return q.SaveWorkerTag(ctx, params)
	})
	return workerTagError(err, "saving worker tag")
}

// DeleteWorkerTag deletes the given tag, after unassigning all workers from it.
func (db *DB) DeleteWorkerTag(ctx context.Context, uuid string) error {
	// As a safety measure, refuse to delete unless foreign key constraints are active.
	fkEnabled, err := db.areForeignKeysEnabled(ctx)
	switch {
	case err != nil:
		return err
	case !fkEnabled:
		return ErrDeletingWithoutFK
	}

	var rowsUpdated int64
	err = db.queriesRW(ctx, func(q *sqlc.Queries) (err error) {
		rowsUpdated, err = q.DeleteWorkerTag(ctx, uuid)
		return
	})
	switch {
	case err != nil:
		return workerTagError(err, "deleting worker tag")
	case rowsUpdated == 0:
		return ErrWorkerTagNotFound
	}

	return nil
}

func (db *DB) FetchWorkerTags(ctx context.Context) ([]WorkerTag, error) {
	var tags []WorkerTag
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		tags, err = q.FetchWorkerTags(ctx)
		return
	})
	return tags, workerTagError(err, "fetching all worker tags")
}

func (db *DB) WorkerSetTags(ctx context.Context, worker *Worker, tagUUIDs []string) error {
	return db.queriesRW(ctx, func(q *sqlc.Queries) error {
		tags, err := q.FetchWorkerTagsByUUIDs(ctx, tagUUIDs)
		if err != nil {
			return workerTagError(err, "fetching worker tags")
		}

		err = q.WorkerRemoveTagMemberships(ctx, worker.ID)
		if err != nil {
			return workerTagError(err, "un-assigning existing worker tags")
		}

		for _, tag := range tags {
			err = q.WorkerAddTagMembership(ctx, sqlc.WorkerAddTagMembershipParams{
				WorkerID:    worker.ID,
				WorkerTagID: tag.ID,
			})
			if err != nil {
				return workerTagError(err, "assigning worker tags")
			}
		}
		return nil
	})
}

func (db *DB) FetchTagsOfWorker(ctx context.Context, workerUUID string) ([]WorkerTag, error) {
	var tags []WorkerTag
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		tags, err = q.FetchTagsOfWorker(ctx, workerUUID)
		return
	})
	return tags, workerTagError(err, "fetching tags of worker %s", workerUUID)
}
