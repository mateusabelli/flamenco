// Package persistence provides the database interface for Flamenco Manager.
package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"fmt"
	"os"
	"time"
)

// Change this to a filename if you want to run a single test and inspect the
// resulting database.
const TestDSN = "file::memory:"

func createTestDB() (db *DB, closer func()) {
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

	err = db.migrate(ctx)
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
	db, dbCloser := createTestDB()

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
