package worker

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// `move-directory` tests.

type cmdMoveDirFixture struct {
	mockCtrl *gomock.Controller
	ce       *CommandExecutor
	mocks    *CommandExecutorMocks
	ctx      context.Context

	temppath string
	cwd      string
}

const (
	taskID     = "90e9d656-e201-4ef0-b6b0-c80684fafa27"
	sourcePath = "render/output/here__intermediate"
	destPath   = "render/output/here"
)

func TestCmdMoveDirectoryNonExistentSourceDir(t *testing.T) {
	f := newCmdMoveDirectoryFixture(t)
	defer f.finish(t)

	f.mocks.listener.EXPECT().LogProduced(gomock.Any(), taskID,
		"move-directory: source path \"render/output/here__intermediate\" does not exist, not moving anything")
	err := f.run()
	var paramErr ParameterInvalidError
	if assert.ErrorAs(t, err, &paramErr) {
		assert.Equal(t, "src", paramErr.Parameter)
		assert.Equal(t, "path does not exist", paramErr.Message)
	}
}

func TestCmdMoveDirectoryHappy(t *testing.T) {
	f := newCmdMoveDirectoryFixture(t)
	defer f.finish(t)

	ensureDirExists(sourcePath)
	fileCreateEmpty(filepath.Join(sourcePath, "testfile.txt"))

	f.mocks.listener.EXPECT().LogProduced(gomock.Any(), taskID,
		"move-directory: moving \"render/output/here__intermediate\" to \"render/output/here\"")
	err := f.run()
	assert.NoError(t, err)

	assert.NoDirExists(t, sourcePath)
	assert.DirExists(t, destPath)
	assert.NoFileExists(t, filepath.Join(sourcePath, "testfile.txt"))
	assert.FileExists(t, filepath.Join(destPath, "testfile.txt"))
}

func TestCmdMoveDirectoryExistingDest(t *testing.T) {
	f := newCmdMoveDirectoryFixture(t)
	defer f.finish(t)

	mtime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05-07:00")
	assert.NoError(t, err)

	ensureDirExists(sourcePath)
	ensureDirExists(destPath)
	fileCreateEmpty(filepath.Join(sourcePath, "sourcefile.txt"))
	fileCreateEmpty(filepath.Join(destPath, "destfile.txt"))

	// Change the atime/mtime of the directory after creating the files, otherwise
	// it'll reset to "now".
	if err := os.Chtimes(destPath, mtime, mtime); err != nil {
		t.Fatalf("changing dir time: %v", err)
	}

	// This cannot be a hard-coded string, as the test would fail in other timezones.
	backupDir := destPath + "-" + mtime.Local().Format("2006-01-02_150405")

	// Just a sanity check.
	ts, err := timestampedPath(destPath)
	assert.NoError(t, err)
	if !assert.Equal(t, backupDir, ts, "the test's sanity check failed") {
		t.FailNow()
	}

	f.mocks.listener.EXPECT().LogProduced(gomock.Any(), taskID,
		fmt.Sprintf("move-directory: moving \"render/output/here\" to %q", backupDir))
	f.mocks.listener.EXPECT().LogProduced(gomock.Any(), taskID,
		"move-directory: moving \"render/output/here__intermediate\" to \"render/output/here\"")

	assert.NoError(t, f.run())

	assert.NoDirExists(t, sourcePath)
	assert.DirExists(t, destPath)
	assert.DirExists(t, backupDir, "old dest dir should have been moved to new location")
	assert.NoFileExists(t, filepath.Join(sourcePath, "sourcefile.txt"))
	assert.FileExists(t, filepath.Join(destPath, "sourcefile.txt"))
	assert.FileExists(t, filepath.Join(backupDir, "destfile.txt"))
}

func TestCmdMoveDirectoryExistingDestAndBackup(t *testing.T) {
	f := newCmdMoveDirectoryFixture(t)
	defer f.finish(t)

	mtime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05-07:00")
	assert.NoError(t, err)

	ensureDirExists(sourcePath)
	ensureDirExists(destPath)
	fileCreateEmpty(filepath.Join(sourcePath, "sourcefile.txt"))
	fileCreateEmpty(filepath.Join(destPath, "destfile.txt"))

	// This cannot be a hard-coded string, as the test would fail in other timezones.
	backupDir := destPath + "-" + mtime.Local().Format("2006-01-02_150405")
	ensureDirExists(backupDir)
	ensureDirExists(backupDir + "-046")
	fileCreateEmpty(filepath.Join(backupDir, "backupfile.txt"))

	// uniqueDir is where 'dest' will end up, because 'backupDir' already existed beforehand.
	uniqueDir := backupDir + "-047"

	// Change the atime/mtime of the directory after creating the files, otherwise
	// it'll reset to "now".
	if err := os.Chtimes(destPath, mtime, mtime); err != nil {
		t.Fatalf("changing dir time: %v", err)
	}

	f.mocks.listener.EXPECT().LogProduced(gomock.Any(), taskID,
		fmt.Sprintf("move-directory: moving \"render/output/here\" to %q", uniqueDir))
	f.mocks.listener.EXPECT().LogProduced(gomock.Any(), taskID,
		"move-directory: moving \"render/output/here__intermediate\" to \"render/output/here\"")

	assert.NoError(t, f.run())

	assert.NoDirExists(t, sourcePath)
	assert.DirExists(t, destPath)
	assert.DirExists(t, backupDir, "the backup directory should not have been removed")
	assert.DirExists(t, uniqueDir, "old dest dir should have been moved to new unique location")

	assert.NoFileExists(t, filepath.Join(sourcePath, "sourcefile.txt"))
	assert.FileExists(t, filepath.Join(destPath, "sourcefile.txt"))
	assert.FileExists(t, filepath.Join(backupDir, "backupfile.txt"), "the original backup directory should not have been touched")
	assert.FileExists(t, filepath.Join(uniqueDir, "destfile.txt"), "the dest dir should have been moved to a unique dir")
}

func TestTimestampedPathFile(t *testing.T) {
	f := newCmdMoveDirectoryFixture(t)
	defer f.finish(t)

	mtime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05-07:00")
	assert.NoError(t, err)

	fileCreateEmpty("somefile.txt")
	if err := os.Chtimes("somefile.txt", mtime, mtime); err != nil {
		t.Fatal(err.Error())
	}

	newpath, err := timestampedPath("somefile.txt")

	// This cannot be a hard-coded string, as the test would fail in other timezones.
	expect := "somefile.txt-" + mtime.Local().Format("2006-01-02_150405")

	assert.NoError(t, err)
	assert.Equal(t, expect, newpath)
}

func TestTimestampedPathDir(t *testing.T) {
	f := newCmdMoveDirectoryFixture(t)
	defer f.finish(t)

	mtime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05-07:00")
	assert.NoError(t, err)

	if err := os.Mkdir("somedir", os.ModePerm); err != nil {
		t.Fatal(err.Error())
	}
	if err := os.Chtimes("somedir", mtime, mtime); err != nil {
		t.Fatal(err.Error())
	}

	newpath, err := timestampedPath("somedir")

	// This cannot be a hard-coded string, as the test would fail in other timezones.
	expect := "somedir-" + mtime.Local().Format("2006-01-02_150405")

	assert.NoError(t, err)
	assert.Equal(t, expect, newpath)
}

func TestUniquePath(t *testing.T) {
	f := newCmdMoveDirectoryFixture(t)
	defer f.finish(t)

	fileCreateEmpty("thefile.txt")
	fileCreateEmpty("thefile.txt-1")
	fileCreateEmpty("thefile.txt-003")
	fileCreateEmpty("thefile.txt-46")

	newpath, err := uniquePath("thefile.txt")
	assert.NoError(t, err)
	assert.Equal(t, "thefile.txt-047", newpath)

	// Test with existing suffix longer than 3 digits.
	fileCreateEmpty("thefile.txt-10327")
	newpath, err = uniquePath("thefile.txt")
	assert.NoError(t, err)
	assert.Equal(t, "thefile.txt-10328", newpath)
}

func newCmdMoveDirectoryFixture(t *testing.T) cmdMoveDirFixture {
	mockCtrl := gomock.NewController(t)
	ce, mocks := testCommandExecutor(t, mockCtrl)

	temppath, err := os.MkdirTemp("", "test-move-directory")
	if err != nil {
		t.Fatalf("unable to create temp dir: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getcw: %v", err)
	}

	if err := os.Chdir(temppath); err != nil {
		t.Fatalf("chdir(%s): %v", temppath, err)
	}

	return cmdMoveDirFixture{
		mockCtrl: mockCtrl,
		ce:       ce,
		mocks:    mocks,
		ctx:      context.Background(),
		temppath: temppath,
		cwd:      cwd,
	}
}

func (f cmdMoveDirFixture) finish(t *testing.T) {
	if err := os.Chdir(f.cwd); err != nil {
		t.Fatalf("chdir(%s): %v", f.cwd, err)
	}

	os.RemoveAll(f.temppath)
	f.mockCtrl.Finish()
}

func (f cmdMoveDirFixture) run() error {
	cmd := api.Command{
		Name: "move-directory",
		Parameters: map[string]interface{}{
			"src":  sourcePath,
			"dest": destPath,
		},
	}
	return f.ce.Run(f.ctx, taskID, cmd)
}

// `copy-file` tests.

type cmdCopyFileFixture struct {
	mockCtrl *gomock.Controller
	ce       *CommandExecutor
	mocks    *CommandExecutorMocks
	ctx      context.Context

	temppath string
	cwd      string

	absolute_src_path  string
	absolute_dest_path string
}

func TestCmdCopyFile(t *testing.T) {
	f := newCmdCopyFileFixture(t)
	defer f.finish(t)

	src_dirpath := filepath.Join(f.temppath, "src_path/to")
	dest_dirpath := filepath.Join(f.temppath, "dest_path/to")

	f.absolute_src_path = filepath.Join(src_dirpath, "file1.txt")
	f.absolute_dest_path = filepath.Join(dest_dirpath, "file2.txt")

	directoryEnsureExist(src_dirpath)
	assert.DirExists(t, src_dirpath)

	fileCreateEmpty(f.absolute_src_path)
	assert.FileExists(t, f.absolute_src_path)

	assert.NoDirExists(t, dest_dirpath)
	assert.NoFileExists(t, f.absolute_dest_path)

	f.mocks.listener.EXPECT().LogProduced(gomock.Any(), taskID,
		fmt.Sprintf("copy-file: copying %q to %q", f.absolute_src_path, f.absolute_dest_path))

	f.mocks.listener.EXPECT().LogProduced(gomock.Any(), taskID,
		fmt.Sprintf("copy-file: copied %q to %q", f.absolute_src_path, f.absolute_dest_path))

	assert.NoError(t, f.run())

	assert.DirExists(t, src_dirpath)
	assert.DirExists(t, dest_dirpath)
	assert.FileExists(t, f.absolute_src_path)
	assert.FileExists(t, f.absolute_dest_path)
}

func TestCmdCopyFileDestinationExists(t *testing.T) {
	f := newCmdCopyFileFixture(t)
	defer f.finish(t)

	src_dirpath := filepath.Join(f.temppath, "src_path/to")
	dest_dirpath := filepath.Join(f.temppath, "dest_path/to")

	f.absolute_src_path = filepath.Join(src_dirpath, "file1.txt")
	f.absolute_dest_path = filepath.Join(dest_dirpath, "file2.txt")

	directoryEnsureExist(src_dirpath)
	assert.DirExists(t, src_dirpath)

	fileCreateEmpty(f.absolute_src_path)
	assert.FileExists(t, f.absolute_src_path)

	assert.NoDirExists(t, dest_dirpath)
	assert.NoFileExists(t, f.absolute_dest_path)

	directoryEnsureExist(dest_dirpath)
	assert.DirExists(t, dest_dirpath)

	fileCreateEmpty(f.absolute_dest_path)
	assert.FileExists(t, f.absolute_dest_path)

	f.mocks.listener.EXPECT().LogProduced(gomock.Any(), taskID,
		fmt.Sprintf("copy-file: destination path %q already exists, not copying anything", f.absolute_dest_path))

	assert.Error(t, f.run())
}

func TestCmdCopyFileSourceIsDir(t *testing.T) {
	f := newCmdCopyFileFixture(t)
	defer f.finish(t)

	src_dirpath := filepath.Join(f.temppath, "src_path/to")
	dest_dirpath := filepath.Join(f.temppath, "dest_path/to")

	f.absolute_src_path = src_dirpath
	f.absolute_dest_path = filepath.Join(dest_dirpath, "file2.txt")

	directoryEnsureExist(src_dirpath)
	assert.DirExists(t, src_dirpath)

	assert.NoDirExists(t, dest_dirpath)
	assert.NoFileExists(t, f.absolute_dest_path)

	f.mocks.listener.EXPECT().LogProduced(gomock.Any(), taskID,
		fmt.Sprintf("copy-file: copying %q to %q", f.absolute_src_path, f.absolute_dest_path))

	f.mocks.listener.EXPECT().LogProduced(gomock.Any(), taskID,
		fmt.Sprintf("copy-file: invalid source file %q: stat %s: not a regular file",
			f.absolute_src_path, f.absolute_src_path))

	assert.Error(t, f.run())
}

func newCmdCopyFileFixture(t *testing.T) cmdCopyFileFixture {
	mockCtrl := gomock.NewController(t)
	ce, mocks := testCommandExecutor(t, mockCtrl)

	temppath, err := os.MkdirTemp("", "test-copy-file")
	if err != nil {
		t.Fatalf("unable to create temp dir: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getcw: %v", err)
	}

	if err := os.Chdir(temppath); err != nil {
		t.Fatalf("chdir(%s): %v", temppath, err)
	}

	return cmdCopyFileFixture{
		mockCtrl: mockCtrl,
		ce:       ce,
		mocks:    mocks,
		ctx:      context.Background(),
		temppath: temppath,
		cwd:      cwd,
	}
}

func (f cmdCopyFileFixture) finish(t *testing.T) {
	if err := os.Chdir(f.cwd); err != nil {
		t.Fatalf("chdir(%s): %v", f.cwd, err)
	}

	os.RemoveAll(f.temppath)
	f.mockCtrl.Finish()
}

func (f cmdCopyFileFixture) run() error {
	cmd := api.Command{
		Name: "copy-file",
		Parameters: map[string]interface{}{
			"src":  f.absolute_src_path,
			"dest": f.absolute_dest_path,
		},
	}
	return f.ce.Run(f.ctx, taskID, cmd)
}

// Misc utils

func ensureDirExists(dirpath string) {
	if err := os.MkdirAll(dirpath, fs.ModePerm); err != nil {
		panic(fmt.Sprintf("unable to create dir %s: %v", dirpath, err))
	}
}

func fileCreateEmpty(filename string) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_RDONLY, 0666)

	if err != nil {
		panic(err.Error())
	}
	file.Close()
}

func directoryEnsureExist(dirpath string) {
	err := os.MkdirAll(dirpath, 0750)

	if err != nil {
		panic(err.Error())
	}
}
