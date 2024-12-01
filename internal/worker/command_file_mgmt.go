package worker

// SPDX-License-Identifier: GPL-3.0-or-later

/* This file contains the commands in the "file-management" type group. */

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"projects.blender.org/studio/flamenco/pkg/api"
)

// cmdMoveDirectory executes the "move-directory" command.
// It moves directory 'src' to 'dest'; if 'dest' already exists, it's moved to 'dest-{timestamp}'.
func (ce *CommandExecutor) cmdMoveDirectory(ctx context.Context, logger zerolog.Logger, taskID string, cmd api.Command) error {
	var src, dest string
	var ok bool

	if src, ok = cmdParameter[string](cmd, "src"); !ok || src == "" {
		logger.Warn().Interface("command", cmd).Msg("missing 'src' parameter")
		return NewParameterMissingError("src", cmd)
	}
	if dest, ok = cmdParameter[string](cmd, "dest"); !ok || dest == "" {
		logger.Warn().Interface("command", cmd).Msg("missing 'dest' parameter")
		return NewParameterMissingError("dest", cmd)
	}

	logger = logger.With().
		Str("src", src).
		Str("dest", dest).
		Logger()
	if !fileExists(src) {
		logger.Warn().Msg("source path does not exist, not moving anything")
		msg := fmt.Sprintf("%s: source path %q does not exist, not moving anything", cmd.Name, src)
		if err := ce.listener.LogProduced(ctx, taskID, msg); err != nil {
			return err
		}
		return NewParameterInvalidError("src", cmd, "path does not exist")
	}

	if fileExists(dest) {
		backup, err := timestampedPath(dest)
		if err != nil {
			logger.Error().Err(err).Str("path", dest).Msg("unable to determine timestamp of directory")
			return err
		}

		if fileExists(backup) {
			logger.Debug().Str("backup", backup).Msg("backup destination also exists, finding one that does not")
			backup, err = uniquePath(backup)
			if err != nil {
				return err
			}
		}

		logger.Info().
			Str("toBackup", backup).
			Msg("dest directory exists, moving to backup")
		if err := ce.moveAndLog(ctx, taskID, cmd.Name, dest, backup); err != nil {
			return err
		}
	}

	// self._log.info("Moving %s to %s", src, dest)
	// await self.worker.register_log(
	// 		"%s: Moving %s to %s", self.command_name, src, dest
	// )
	// src.rename(dest)
	logger.Info().Msg("moving directory")
	return ce.moveAndLog(ctx, taskID, cmd.Name, src, dest)
}

// cmdCopyFiles executes the "copy-file" command.
// It takes an absolute source and destination file path,
// and copies the source file to its destination, if possible.
// Missing directories in destination path are created as needed.
// If the target path already exists, an error is returned. Destination will not be overwritten.
func (ce *CommandExecutor) cmdCopyFile(ctx context.Context, logger zerolog.Logger, taskID string, cmd api.Command) error {
	var src, dest string
	var ok bool

	logger = logger.With().
		Interface("command", cmd).
		Str("src", src).
		Str("dest", dest).
		Logger()

	if src, ok = cmdParameter[string](cmd, "src"); !ok || src == "" {
		msg := "missing or empty 'src' parameter"
		err := NewParameterMissingError("src", cmd)
		return ce.errorLogProcess(ctx, logger, cmd, taskID, err, msg)
	}
	if !filepath.IsAbs(src) {
		msg := fmt.Sprintf("source path %q is not absolute, not copying anything", src)
		err := NewParameterInvalidError("src", cmd, "path is not absolute")
		return ce.errorLogProcess(ctx, logger, cmd, taskID, err, msg)
	}
	if !fileExists(src) {
		msg := fmt.Sprintf("source path %q does not exist, not copying anything", src)
		err := NewParameterInvalidError("src", cmd, "path does not exist")
		return ce.errorLogProcess(ctx, logger, cmd, taskID, err, msg)
	}

	if dest, ok = cmdParameter[string](cmd, "dest"); !ok || dest == "" {
		msg := "missing or empty 'dest' parameter"
		err := NewParameterMissingError("dest", cmd)
		return ce.errorLogProcess(ctx, logger, cmd, taskID, err, msg)
	}
	if !filepath.IsAbs(dest) {
		msg := fmt.Sprintf("destination path %q is not absolute, not copying anything", src)
		err := NewParameterInvalidError("dest", cmd, "path is not absolute")
		return ce.errorLogProcess(ctx, logger, cmd, taskID, err, msg)
	}
	if fileExists(dest) {
		msg := fmt.Sprintf("destination path %q already exists, not copying anything", dest)
		err := NewParameterInvalidError("dest", cmd, "path already exists")
		return ce.errorLogProcess(ctx, logger, cmd, taskID, err, msg)
	}

	msg := fmt.Sprintf("copying %q to %q", src, dest)
	if err := ce.errorLogProcess(ctx, logger, cmd, taskID, nil, msg); err != nil {
		return err
	}

	logMsg, err := fileCopy(src, dest)
	return ce.errorLogProcess(ctx, logger, cmd, taskID, err, logMsg)
}

func (ce *CommandExecutor) errorLogProcess(ctx context.Context, logger zerolog.Logger, cmd api.Command, taskID string, err error, logMsg string) error {
	msg := fmt.Sprintf("%s: %s", cmd.Name, logMsg)

	if err != nil {
		logger.Warn().Msg(msg)
	} else {
		logger.Info().Msg(msg)
	}

	if logErr := ce.listener.LogProduced(ctx, taskID, msg); logErr != nil {
		return logErr
	}
	return err
}

// moveAndLog renames a file/directory from `src` to `dest`, and logs the moveAndLog.
// The other parameters are just for logging.
func (ce *CommandExecutor) moveAndLog(ctx context.Context, taskID, cmdName, src, dest string) error {
	msg := fmt.Sprintf("%s: moving %q to %q", cmdName, src, dest)
	if err := ce.listener.LogProduced(ctx, taskID, msg); err != nil {
		return err
	}

	if err := os.Rename(src, dest); err != nil {
		msg := fmt.Sprintf("%s: could not move %q to %q: %v", cmdName, src, dest, err)
		if err := ce.listener.LogProduced(ctx, taskID, msg); err != nil {
			return err
		}
		return err
	}

	return nil
}

func fileCopy(src, dest string) (string, error) {
	src_file, err := os.Open(src)
	if err != nil {
		msg := fmt.Sprintf("failed to open source file %q: %v", src, err)
		return msg, err
	}
	defer src_file.Close()

	src_file_stat, err := src_file.Stat()
	if err != nil {
		msg := fmt.Sprintf("failed to stat source file %q: %v", src, err)
		return msg, err
	}

	if !src_file_stat.Mode().IsRegular() {
		err := &os.PathError{Op: "stat", Path: src, Err: errors.New("not a regular file")}
		msg := fmt.Sprintf("invalid source file %q: %v", src, err)
		return msg, err
	}

	dest_dirpath := filepath.Dir(dest)
	if !fileExists(dest_dirpath) {
		if err := os.MkdirAll(dest_dirpath, 0750); err != nil {
			msg := fmt.Sprintf("failed to create directories %q: %v", dest_dirpath, err)
			return msg, err
		}
	}

	dest_file, err := os.Create(dest)
	if err != nil {
		msg := fmt.Sprintf("failed to create destination file %q: %v", dest, err)
		return msg, err
	}
	defer dest_file.Close()

	if _, err := io.Copy(dest_file, src_file); err != nil {
		msg := fmt.Sprintf("failed to copy %q to %q: %v", src, dest, err)
		return msg, err
	}

	msg := fmt.Sprintf("copied %q to %q", src, dest)
	return msg, nil
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !errors.Is(err, fs.ErrNotExist)
}

// timestampedPath returns the path with its modification time appended to the name.
func timestampedPath(filepath string) (string, error) {
	stat, err := os.Stat(filepath)
	if err != nil {
		return "", fmt.Errorf("getting mtime of %s: %w", filepath, err)
	}

	// Round away the milliseconds, as those aren't all that interesting.
	// Uniqueness can ensured by calling unique_path() later.
	mtime := stat.ModTime().Round(time.Second)

	iso := mtime.Local().Format("2006-01-02_150405") // YYYY-MM-DD_HHMMSS
	return fmt.Sprintf("%s-%s", filepath, iso), nil
}

// uniquePath returns the path, or if it exists, the path with a unique suffix.
func uniquePath(path string) (string, error) {
	matches, err := filepath.Glob(path + "-*")
	if err != nil {
		return "", err
	}

	suffixRe, err := regexp.Compile("-([0-9]+)$")
	if err != nil {
		return "", fmt.Errorf("compiling regular expression: %w", err)
	}

	var maxSuffix int64
	for _, path := range matches {
		matches := suffixRe.FindStringSubmatch(path)
		if len(matches) < 2 {
			continue
		}
		suffix := matches[1]
		value, err := strconv.ParseInt(suffix, 10, 64)
		if err != nil {
			// Non-numeric suffixes are fine; they just don't count for this function.
			continue
		}

		if value > maxSuffix {
			maxSuffix = value
		}
	}

	return fmt.Sprintf("%s-%03d", path, maxSuffix+1), nil
}
