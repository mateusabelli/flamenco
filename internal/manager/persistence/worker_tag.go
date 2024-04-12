package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type WorkerTag struct {
	Model

	UUID        string `gorm:"type:char(36);default:'';unique;index"`
	Name        string `gorm:"type:varchar(64);default:'';unique"`
	Description string `gorm:"type:varchar(255);default:''"`

	Workers []*Worker `gorm:"many2many:worker_tag_membership;constraint:OnDelete:CASCADE"`
}

func (db *DB) CreateWorkerTag(ctx context.Context, wc *WorkerTag) error {
	if err := db.gormDB.WithContext(ctx).Create(wc).Error; err != nil {
		return fmt.Errorf("creating new worker tag: %w", err)
	}
	return nil
}

// HasWorkerTags returns whether there are any tags defined at all.
func (db *DB) HasWorkerTags(ctx context.Context) (bool, error) {
	var count int64
	tx := db.gormDB.WithContext(ctx).
		Model(&WorkerTag{}).
		Count(&count)
	if err := tx.Error; err != nil {
		return false, workerTagError(err, "counting worker tags")
	}
	return count > 0, nil
}

func (db *DB) FetchWorkerTag(ctx context.Context, uuid string) (*WorkerTag, error) {
	tx := db.gormDB.WithContext(ctx)
	return fetchWorkerTag(tx, uuid)
}

// fetchWorkerTag fetches the worker tag using the given database instance.
func fetchWorkerTag(gormDB *gorm.DB, uuid string) (*WorkerTag, error) {
	w := WorkerTag{}
	tx := gormDB.First(&w, "uuid = ?", uuid)
	if tx.Error != nil {
		return nil, workerTagError(tx.Error, "fetching worker tag")
	}
	return &w, nil
}

func (db *DB) SaveWorkerTag(ctx context.Context, tag *WorkerTag) error {
	if err := db.gormDB.WithContext(ctx).Save(tag).Error; err != nil {
		return workerTagError(err, "saving worker tag")
	}
	return nil
}

// DeleteWorkerTag deletes the given tag, after unassigning all workers from it.
func (db *DB) DeleteWorkerTag(ctx context.Context, uuid string) error {
	// As a safety measure, refuse to delete unless foreign key constraints are active.
	fkEnabled, err := db.areForeignKeysEnabled()
	if err != nil {
		return fmt.Errorf("checking whether foreign keys are enabled: %w", err)
	}
	if !fkEnabled {
		return ErrDeletingWithoutFK
	}

	tx := db.gormDB.WithContext(ctx).
		Where("uuid = ?", uuid).
		Delete(&WorkerTag{})
	if tx.Error != nil {
		return workerTagError(tx.Error, "deleting worker tag")
	}
	if tx.RowsAffected == 0 {
		return ErrWorkerTagNotFound
	}
	return nil
}

func (db *DB) FetchWorkerTags(ctx context.Context) ([]*WorkerTag, error) {
	tags := make([]*WorkerTag, 0)
	tx := db.gormDB.WithContext(ctx).Model(&WorkerTag{}).Scan(&tags)
	if tx.Error != nil {
		return nil, workerTagError(tx.Error, "fetching all worker tags")
	}
	return tags, nil
}

func (db *DB) fetchWorkerTagsWithUUID(ctx context.Context, tagUUIDs []string) ([]*WorkerTag, error) {
	tags := make([]*WorkerTag, 0)
	tx := db.gormDB.WithContext(ctx).
		Model(&WorkerTag{}).
		Where("uuid in ?", tagUUIDs).
		Scan(&tags)
	if tx.Error != nil {
		return nil, workerTagError(tx.Error, "fetching all worker tags")
	}
	return tags, nil
}

func (db *DB) WorkerSetTags(ctx context.Context, worker *Worker, tagUUIDs []string) error {
	tags, err := db.fetchWorkerTagsWithUUID(ctx, tagUUIDs)
	if err != nil {
		return workerTagError(err, "fetching worker tags")
	}

	err = db.gormDB.WithContext(ctx).
		Model(worker).
		Association("Tags").
		Replace(tags)
	if err != nil {
		return workerTagError(err, "updating worker tags")
	}
	return nil
}
