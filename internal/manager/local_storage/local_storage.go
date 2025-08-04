package local_storage

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/pkg/crosspath"
)

type StorageInfo struct {
	rootPath string
}

// NewNextToExe returns a storage representation that sits next to the
// currently-running executable. If that directory cannot be determined, falls
// back to the current working directory.
// If `subdir` is an absolute path, it is used as-is.
func NewNextToExe(subdir string) StorageInfo {
	if filepath.IsAbs(subdir) {
		return StorageInfo{
			rootPath: subdir,
		}
	}

	exeDir := getSuitableStorageRoot()
	storagePath := filepath.Join(exeDir, subdir)

	return StorageInfo{
		rootPath: storagePath,
	}
}

// Root returns the root path of the storage.
func (si StorageInfo) Root() string {
	return si.rootPath
}

// ForJob returns the absolute directory path for storing job-related files.
func (si StorageInfo) ForJob(jobUUID string) string {
	return filepath.Join(si.rootPath, relPathForJob(jobUUID))
}

func (si StorageInfo) RemoveJobStorage(ctx context.Context, jobUUID string) error {
	path := si.ForJob(jobUUID)
	log.Debug().Str("path", path).Msg("erasing manager-local job storage directory")

	if err := removeDirectory(path); err != nil {
		return fmt.Errorf("unable to erase %q: %w", path, err)
	}

	// The path should be in some intermediate path
	// (`root/intermediate/job-uuid`), which might need removing if it's empty.
	intermediate := filepath.Dir(path)
	if intermediate == si.rootPath {
		// There is no intermediate dir for jobless situations. Or maybe the rest of
		// the code changed since this function was written. Regardless of the
		// reason, this function shouldn't remove the local storage root.
		return nil
	}

	if err := os.Remove(intermediate); err != nil {
		// This is absolutely fine, as it'll happen when the directory is not empty
		// and thus shouldn't be removed anyway.
		log.Trace().
			Str("job", jobUUID).
			Str("path", intermediate).
			AnErr("cause", err).
			Msg("RemoveJobStorage() could not remove intermediate directory, this is fine")
	}
	return nil
}

// Erase removes the entire storage directory from disk.
func (si StorageInfo) Erase() error {
	log.Info().Str("path", si.rootPath).Msg("erasing storage directory")

	if err := removeDirectory(si.rootPath); err != nil {
		return fmt.Errorf("unable to erase %q: %w", si.rootPath, err)
	}
	return nil
}

// MustErase removes the entire storage directory from disk, and panics if it
// cannot do that. This is primarily aimed at cleaning up at the end of unit
// tests.
func (si StorageInfo) MustErase() {
	err := si.Erase()
	if err != nil {
		panic(err)
	}
}

// RelPath tries to make the given path relative to the local storage root.
// Assumes `path` is already an absolute path.
func (si StorageInfo) RelPath(path string) (string, error) {
	return filepath.Rel(si.rootPath, path)
}

// Returns a sub-directory suitable for files of this job.
// Note that this is intentionally in sync with the `filepath()` function in
// `internal/manager/task_logs/task_logs.go`.
func relPathForJob(jobUUID string) string {
	if jobUUID == "" {
		return "jobless"
	}
	return filepath.Join("job-"+jobUUID[:4], jobUUID)
}

func getSuitableStorageRoot() string {
	exename, err := os.Executable()
	if err == nil {
		return filepath.Dir(exename)
	}
	log.Error().Err(err).Msg("unable to determine the path of the currently running executable")

	// Fall back to current working directory.
	cwd, err := os.Getwd()
	if err == nil {
		return cwd
	}
	log.Error().Err(err).Msg("unable to determine the current working directory")

	// Fall back to "." if all else fails.
	return "."
}

// removeDirectory removes the given path, but only if it is not a root path and
// not the user's home directory.
func removeDirectory(path string) error {
	if path == "" {
		return fmt.Errorf("refusing to erase empty directory path (%q)", path)
	}
	if crosspath.IsRoot(path) {
		return errors.New("refusing to erase root directory")
	}
	if home, found := os.LookupEnv("HOME"); found && home == path {
		return errors.New("refusing to erase home directory")
	}

	log.Debug().Str("path", path).Msg("erasing directory")
	return os.RemoveAll(path)
}
