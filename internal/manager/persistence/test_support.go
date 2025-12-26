// Package persistence provides the database interface for Flamenco Manager.
package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// Change this to a filename if you want to run a single test and inspect the
// resulting database.
const TestDSN = "file::memory:"

func CreateTestDB() (db *DB, closer func()) {
	// Delete the SQLite file if it exists on disk.
	if _, err := os.Stat(TestDSN); err == nil {
		if err := os.Remove(TestDSN); err != nil {
			panic(fmt.Sprintf("unable to remove %s: %v", TestDSN, err))
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var err error

	db, err = openDB(ctx, TestDSN)
	if err != nil {
		panic(fmt.Sprintf("opening DB: %v", err))
	}

	_, err = db.migrate(ctx)
	if err != nil {
		panic(fmt.Sprintf("migrating DB: %v", err))
	}

	closer = func() {
		if err := db.Close(); err != nil {
			panic(fmt.Sprintf("closing DB: %v", err))
		}
	}

	return db, closer
}

// persistenceTestFixtures creates a test database and returns it and a context.
// Tests should call the returned cancel function when they're done.
func persistenceTestFixtures(testContextTimeout time.Duration) (context.Context, context.CancelFunc, *DB) {
	db, dbCloser := CreateTestDB()

	var (
		ctx       context.Context
		ctxCancel context.CancelFunc
	)
	if testContextTimeout > 0 {
		ctx, ctxCancel = context.WithTimeout(context.Background(), testContextTimeout)
	} else {
		ctx = context.Background()
		ctxCancel = func() {}
	}

	cancel := func() {
		ctxCancel()
		dbCloser()
	}

	return ctx, cancel, db
}

type WorkerTestFixture struct {
	db   *DB
	ctx  context.Context
	done func()

	worker *Worker
	tag    *WorkerTag
}

func workerTestFixtures(t *testing.T, testContextTimeout time.Duration) WorkerTestFixture {
	ctx, cancel, db := persistenceTestFixtures(testContextTimeout)

	w := Worker{
		UUID:               "557930e7-5b55-469e-a6d7-fc800f3685be",
		Name:               "дрон",
		Address:            "fe80::5054:ff:fede:2ad7",
		Platform:           "linux",
		Software:           "3.0",
		Status:             api.WorkerStatusAwake,
		SupportedTaskTypes: "blender,ffmpeg,file-management",
	}

	wc := WorkerTag{
		UUID:        "e0e05417-9793-4829-b1d0-d446dd819f3d",
		Name:        "arbejdsklynge",
		Description: "Worker tag in Danish",
	}

	require.NoError(t, db.CreateWorker(ctx, &w))
	require.NoError(t, db.CreateWorkerTag(ctx, &wc))

	return WorkerTestFixture{
		db:   db,
		ctx:  ctx,
		done: cancel,

		worker: &w,
		tag:    &wc,
	}
}
