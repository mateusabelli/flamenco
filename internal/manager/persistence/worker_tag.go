package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"fmt"

	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
)

type WorkerTag = sqlc.WorkerTag

func (db *DB) CreateWorkerTag(ctx context.Context, tag *WorkerTag) error {
	queries := db.queries()

	now := db.now()
	dbID, err := queries.CreateWorkerTag(ctx, sqlc.CreateWorkerTagParams{
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
}

// HasWorkerTags returns whether there are any tags defined at all.
func (db *DB) HasWorkerTags(ctx context.Context) (bool, error) {
	queries := db.queries()

	count, err := queries.CountWorkerTags(ctx)
	if err != nil {
		return false, workerTagError(err, "counting worker tags")
	}

	return count > 0, nil
}

func (db *DB) FetchWorkerTag(ctx context.Context, uuid string) (WorkerTag, error) {
	queries := db.queries()

	workerTag, err := queries.FetchWorkerTagByUUID(ctx, uuid)
	if err != nil {
		return WorkerTag{}, workerTagError(err, "fetching worker tag")
	}

	return workerTag, nil
}

func (db *DB) FetchWorkerTagByName(ctx context.Context, name string) (WorkerTag, error) {
	queries := db.queries()

	workerTag, err := queries.FetchWorkerTagByName(ctx, name)
	if err != nil {
		return WorkerTag{}, workerTagError(err, "fetching worker tag")
	}

	return workerTag, nil
}

func (db *DB) FetchWorkerTagByID(ctx context.Context, id int64) (WorkerTag, error) {
	queries := db.queries()
	return fetchWorkerTagByID(ctx, queries, id)
}

// fetchWorkerTagByID fetches the worker tag using the given database instance.
func fetchWorkerTagByID(ctx context.Context, queries *sqlc.Queries, id int64) (WorkerTag, error) {
	workerTag, err := queries.FetchWorkerTagByID(ctx, id)
	if err != nil {
		return WorkerTag{}, workerTagError(err, "fetching worker tag")
	}

	return workerTag, nil
}

func (db *DB) SaveWorkerTag(ctx context.Context, tag *WorkerTag) error {
	queries := db.queries()

	err := queries.SaveWorkerTag(ctx, sqlc.SaveWorkerTagParams{
		UpdatedAt:   db.nowNullable(),
		UUID:        tag.UUID,
		Name:        tag.Name,
		Description: tag.Description,
		WorkerTagID: tag.ID,
	})
	if err != nil {
		return workerTagError(err, "saving worker tag")
	}
	return nil
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

	queries := db.queries()

	rowsUpdated, err := queries.DeleteWorkerTag(ctx, uuid)
	switch {
	case err != nil:
		return workerTagError(err, "deleting worker tag")
	case rowsUpdated == 0:
		return ErrWorkerTagNotFound
	}

	return nil
}

func (db *DB) FetchWorkerTags(ctx context.Context) ([]WorkerTag, error) {
	queries := db.queries()

	tags, err := queries.FetchWorkerTags(ctx)
	if err != nil {
		return nil, workerTagError(err, "fetching all worker tags")
	}
	return tags, nil
}

func (db *DB) fetchWorkerTagsWithUUID(
	ctx context.Context,
	queries *sqlc.Queries,
	tagUUIDs []string,
) ([]WorkerTag, error) {
	tags, err := queries.FetchWorkerTagsByUUIDs(ctx, tagUUIDs)
	if err != nil {
		return nil, workerTagError(err, "fetching all worker tags")
	}
	return tags, nil
}

func (db *DB) WorkerSetTags(ctx context.Context, worker *Worker, tagUUIDs []string) error {
	qtx, err := db.queriesWithTX()
	if err != nil {
		return err
	}
	defer qtx.rollback()

	tags, err := db.fetchWorkerTagsWithUUID(ctx, qtx.queries, tagUUIDs)
	if err != nil {
		return workerTagError(err, "fetching worker tags")
	}

	err = qtx.queries.WorkerRemoveTagMemberships(ctx, int64(worker.ID))
	if err != nil {
		return workerTagError(err, "un-assigning existing worker tags")
	}

	for _, tag := range tags {
		err = qtx.queries.WorkerAddTagMembership(ctx, sqlc.WorkerAddTagMembershipParams{
			WorkerID:    int64(worker.ID),
			WorkerTagID: int64(tag.ID),
		})
		if err != nil {
			return workerTagError(err, "assigning worker tags")
		}
	}

	return qtx.commit()
}

func (db *DB) FetchTagsOfWorker(ctx context.Context, workerUUID string) ([]WorkerTag, error) {
	queries := db.queries()
	tags, err := queries.FetchTagsOfWorker(ctx, workerUUID)
	if err != nil {
		return nil, workerTagError(err, "fetching tags of worker %s", workerUUID)
	}
	return tags, nil
}
