//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"
)

// Remove executables and other build output
func Clean() error {
	if err := cleanWebappStatic(); err != nil {
		return err
	}

	if err := sh.Run("go", "clean"); err != nil {
		return err
	}

	if err := rm(
		"flamenco-manager", "flamenco-manager.exe",
		"flamenco-manager_race", "flamenco-manager_race.exe",
		"flamenco-worker", "flamenco-worker.exe",
		"flamenco-worker_race", "flamenco-worker_race.exe",
	); err != nil {
		return err
	}
	return nil
}

func cleanWebappStatic() error {
	// Just a simple heuristic to avoid deleting things like "/" or "C:\"
	if len(webStatic) < 4 {
		panic(fmt.Sprintf("webStatic path is too short, I don't trust it: %q", webStatic))
	}

	if err := sh.Rm(webStatic); err != nil {
		return fmt.Errorf("unable to remove old web static dir %q: %w", webStatic, err)
	}
	if err := os.MkdirAll(webStatic, os.ModePerm); err != nil {
		return fmt.Errorf("unable to create web static dir %q: %w", webStatic, err)
	}

	// Make sure there is at least something to embed by Go, or it may cause some
	// errors. This is done in the 'clean' function so that the Go code can be
	// built before building the webapp.
	emptyfile := filepath.Join(webStatic, "emptyfile")
	if err := os.WriteFile(emptyfile, []byte{}, os.ModePerm); err != nil {
		return err
	}
	return nil
}

func rm(path ...string) error {
	for _, p := range path {
		if err := sh.Rm(p); err != nil {
			return err
		}
	}
	return nil
}
