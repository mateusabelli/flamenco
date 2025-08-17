package worker

// SPDX-License-Identifier: GPL-3.0-or-later

/* This file contains the commands in the "blender" type group. */

import (
	"context"
	"os/exec"
	"regexp"
	"sync"

	"github.com/google/shlex"
	"github.com/rs/zerolog"

	"projects.blender.org/studio/flamenco/internal/find_blender"
	"projects.blender.org/studio/flamenco/pkg/api"
	"projects.blender.org/studio/flamenco/pkg/crosspath"
)

var (
	regexpFileSaved     = regexp.MustCompile(`^Saved: '(.*)'`)
	regexpFrameRendered = regexp.MustCompile(`^Time: ([0-9:.]+)`)
	regexpFrameSkipped  = regexp.MustCompile(`^skipping existing frame`)
)

type BlenderParameters struct {
	exe        string   // Expansion of `{blender}`: the executable path defined by the Manager.
	exeArgs    string   // Expansion of `{blenderargs}`: its CLI parameters defined by the Manager.
	argsBefore []string // Additional CLI arguments defined by the job compiler script, to go before the blend file name.
	blendfile  string   // Path of the file to open.
	args       []string // Additional CLI arguments defined by the job compiler script, to go after the blend file name.
}

// cmdBlender executes the "blender-render" command.
func (ce *CommandExecutor) cmdBlenderRender(ctx context.Context, logger zerolog.Logger, taskID string, cmd api.Command) error {
	cmdCtx, cmdCtxCancel := context.WithCancel(ctx)
	defer cmdCtxCancel() // Ensure the subprocess exits whenever this function returns.

	execCmd, err := ce.cmdBlenderRenderCommand(cmdCtx, logger, taskID, cmd)
	if err != nil {
		return err
	}

	logChunker := NewLogChunker(taskID, ce.listener, ce.timeService)
	lineChannel := make(chan string)

	// Process the output of Blender in its own goroutine.
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for line := range lineChannel {
			ce.processLineBlender(ctx, logger, taskID, line)
		}
	}()

	// Run the subprocess.
	subprocessErr := ce.cli.RunWithTextOutput(ctx,
		logger,
		execCmd,
		logChunker,
		lineChannel,
	)

	// Wait for the processing to stop.
	close(lineChannel)
	wg.Wait()

	if subprocessErr != nil {
		logger.Error().Err(subprocessErr).
			Int("exitCode", execCmd.ProcessState.ExitCode()).
			Msg("command exited abnormally")
		return subprocessErr
	}

	return nil
}

func (ce *CommandExecutor) cmdBlenderRenderCommand(
	ctx context.Context,
	logger zerolog.Logger,
	taskID string,
	cmd api.Command,
) (*exec.Cmd, error) {
	parameters, err := cmdBlenderRenderParams(logger, cmd)
	if err != nil {
		return nil, err
	}

	if crosspath.Dir(parameters.exe) == "." {
		// No directory path given. Check that the executable can be found on the
		// path.
		if _, err := exec.LookPath(parameters.exe); err != nil {
			// Attempt a platform-specific way to find which Blender executable to
			// use. If Blender cannot not be found, just use the configured command
			// and let the OS produce the errors.
			path, err := find_blender.FileAssociation()
			if err == nil {
				logger.Info().Str("path", path).Msg("found Blender")
				parameters.exe = path
			}
		}
	}

	cliArgs := make([]string, 0)
	cliArgs = append(cliArgs, parameters.argsBefore...)
	if parameters.blendfile != "" {
		// Only include the blendfile if the parameter is not empty. This makes it
		// possible to pass a Python script that loads/constructs the blend file,
		// instead of loading one explicitly here.
		cliArgs = append(cliArgs, parameters.blendfile)
	}
	cliArgs = append(cliArgs, parameters.args...)
	execCmd := ce.cli.CommandContext(ctx, parameters.exe, cliArgs...)
	if execCmd == nil {
		logger.Error().
			Str("cmdName", cmd.Name).
			Msg("unable to create command executor")
		return nil, ErrNoExecCmd
	}

	return execCmd, nil
}

func cmdBlenderRenderParams(logger zerolog.Logger, cmd api.Command) (BlenderParameters, error) {
	var (
		parameters BlenderParameters
		ok         bool
	)

	if parameters.exe, ok = cmdParameter[string](cmd, "exe"); !ok || parameters.exe == "" {
		logger.Warn().Interface("command", cmd).Msg("missing 'exe' parameter")
		return parameters, NewParameterMissingError("exe", cmd)
	}

	// Ignore the `ok` return value, as a missing exeArgs key is fine:
	parameters.exeArgs, _ = cmdParameter[string](cmd, "exeArgs")

	if parameters.argsBefore, ok = cmdParameterAsStrings(cmd, "argsBefore"); !ok {
		logger.Warn().Interface("command", cmd).Msg("invalid 'argsBefore' parameter")
		return parameters, NewParameterInvalidError("argsBefore", cmd, "cannot convert to list of strings")
	}
	if parameters.blendfile, ok = cmdParameter[string](cmd, "blendfile"); !ok {
		logger.Warn().Interface("command", cmd).Msg("invalid 'blendfile' parameter")
		return parameters, NewParameterInvalidError("blendfile", cmd, "cannot convert to string")
	}
	if parameters.args, ok = cmdParameterAsStrings(cmd, "args"); !ok {
		logger.Warn().Interface("command", cmd).Msg("invalid 'args' parameter")
		return parameters, NewParameterInvalidError("args", cmd, "cannot convert to list of strings")
	}

	// Split the exeArgs string into separate parts, and prepend them to `argsBefore`.
	exeArgs, err := shlex.Split(parameters.exeArgs)
	if err != nil {
		logger.Warn().Err(err).Interface("command", cmd).Msg("error parsing 'exeArgs' parameter with shlex")
		return parameters, NewParameterInvalidError("exeArgs", cmd, err.Error())
	}
	if len(exeArgs) > 0 {
		allArgsBefore := []string{}
		allArgsBefore = append(allArgsBefore, exeArgs...)
		allArgsBefore = append(allArgsBefore, parameters.argsBefore...)
		parameters.argsBefore = allArgsBefore
	}

	return parameters, nil
}

func (ce *CommandExecutor) processLineBlender(ctx context.Context, logger zerolog.Logger, taskID string, line string) {
	// TODO: check for "Warning: Unable to open" and other indicators of missing
	// files. Flamenco v2 updated the task.Activity field for such situations.

	{ // Handle "output produced" indicator.
		match := regexpFileSaved.FindStringSubmatch(line)
		if len(match) >= 2 {
			filename := match[1]

			logger = logger.With().Str("outputFile", filename).Logger()
			logger.Debug().Msg("blender-render: output produced")

			err := ce.listener.OutputProduced(ctx, taskID, filename)
			if err != nil {
				logger.Warn().Err(err).Msg("error submitting produced output to listener")
			}
		}
	}

	{ // Handle "frame done rendering" indicator.
		match := regexpFrameRendered.FindStringSubmatch(line)
		if len(match) >= 2 {
			// TODO: send a "frame rendered" statistics event, with the frame duration.
			duration := match[1]
			logger.Info().
				Str("duration", duration).
				Msg("blender-render: frame rendered")

			err := ce.listener.TaskStep(ctx, taskID)
			if err != nil {
				logger.Warn().Err(err).Msg("error submitting task progress to listener")
			}
		}
	}

	{ // Handle "frame skipped" indicator.
		match := regexpFrameSkipped.FindStringSubmatch(line)
		if len(match) > 0 {
			logger.Info().Msg("blender-render: frame skipped")

			err := ce.listener.TaskStep(ctx, taskID)
			if err != nil {
				logger.Warn().Err(err).Msg("error submitting task progress to listener")
			}
		}
	}

}
