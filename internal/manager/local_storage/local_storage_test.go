package local_storage

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNextToExe(t *testing.T) {
	si := NewNextToExe("nø ASCïÏ")

	// Unit test executables typically are in `/tmp/go-build{random number}`.
	assert.Contains(t, si.rootPath, "go-build")
	assert.Equal(t, filepath.Base(si.rootPath), "nø ASCïÏ",
		"the real path should end in the given directory name")
}

func TestNewNextToExe_noSubdir(t *testing.T) {
	exePath, err := os.Executable()
	require.NoError(t, err)
	exeName := filepath.Base(exePath)

	// The filesystem in an empty "subdirectory" next to the executable should
	// contain the executable.
	si := NewNextToExe("")
	_, err = os.Stat(filepath.Join(si.rootPath, exeName))
	require.NoErrorf(t, err, "should be able to stat this executable %s", exeName)
}

func TestForJob(t *testing.T) {
	si := NewNextToExe("task-logs")
	jobPath := si.ForJob("08e126ef-d773-468b-8bab-19a8213cf2ff")

	expectedSuffix := filepath.Join("task-logs", "job-08e1", "08e126ef-d773-468b-8bab-19a8213cf2ff")
	hasSuffix := strings.HasSuffix(jobPath, expectedSuffix)
	assert.Truef(t, hasSuffix, "expected %s to have suffix %s", jobPath, expectedSuffix)
}

func TestErase(t *testing.T) {
	si := NewNextToExe("task-logs")
	assert.NoDirExists(t, si.rootPath, "creating a StorageInfo should not create the directory")

	jobPath := si.ForJob("08e126ef-d773-468b-8bab-19a8213cf2ff")
	assert.NoDirExists(t, jobPath, "getting a path should not create it")

	require.NoError(t, os.MkdirAll(jobPath, os.ModePerm))
	assert.DirExists(t, jobPath, "os.MkdirAll is borked")

	require.NoError(t, si.Erase())
	assert.NoDirExists(t, si.rootPath, "Erase() should erase the root path, and everything in it")
}

func TestRemoveJobStorage(t *testing.T) {
	si := NewNextToExe("task-logs")

	jobUUID := "08e126ef-d773-468b-8bab-19a8213cf2ff"
	jobPath := si.ForJob(jobUUID)
	assert.NoDirExists(t, jobPath, "getting a path should not create it")

	require.NoError(t, os.MkdirAll(jobPath, os.ModePerm))
	assert.DirExists(t, jobPath, "os.MkdirAll is borked")

	taskFile := filepath.Join(jobPath, "task-07c33f32-b345-4da9-8834-9c91532cd97e.txt")
	require.NoError(t, os.WriteFile(taskFile, []byte("dummy task log"), 0o777))

	require.NoError(t, si.RemoveJobStorage(context.Background(), jobUUID))
	assert.NoDirExists(t, jobPath, "RemoveJobStorage() should erase the entire job-specific storage dir, and everything in it")

	// See if the test assumption (that job dir is in another sub-dir of the root,
	// `root/job-xxyy/xxyyzzblablabla`) still holds.
	intermediate := filepath.Dir(jobPath)
	require.NotEqual(t, si.rootPath, intermediate,
		"Expected job path %s to be in child directory of root %s", jobPath, si.rootPath)

	assert.NoDirExists(t, intermediate, "RemoveJobStorage() should remove empty intermediate paths")
	assert.DirExists(t, si.rootPath, "RemoveJobStorage() should keep the root path")
}

func TestRemoveJobStorageWithoutJobUUID(t *testing.T) {
	si := NewNextToExe("task-logs")

	jobPath := si.ForJob("")
	assert.NoDirExists(t, jobPath, "getting a path should not create it")

	require.NoError(t, os.MkdirAll(jobPath, os.ModePerm))
	assert.DirExists(t, jobPath, "os.MkdirAll is borked")

	taskFile := filepath.Join(jobPath, "task-07c33f32-b345-4da9-8834-9c91532cd97e.txt")
	require.NoError(t, os.WriteFile(taskFile, []byte("dummy task log"), 0o777))

	require.NoError(t, si.RemoveJobStorage(context.Background(), ""))
	assert.NoDirExists(t, jobPath, "RemoveJobStorage() should erase the entire job-specific storage dir, and everything in it")

	// See if the test assumption (that a jobless dir is directly inside the root) still holds.
	intermediate := filepath.Dir(jobPath)
	require.Equal(t, si.rootPath, intermediate,
		"Expected job path %s to be a direct child of root %s", jobPath, si.rootPath)
	assert.DirExists(t, si.rootPath, "RemoveJobStorage() should keep the root path")
}
