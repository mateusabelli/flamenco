// Package api_impl implements the OpenAPI API from pkg/api/flamenco-openapi.yaml.
package api_impl

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"projects.blender.org/studio/flamenco/internal/appinfo"
	"projects.blender.org/studio/flamenco/internal/find_blender"
	"projects.blender.org/studio/flamenco/internal/manager/config"
	"projects.blender.org/studio/flamenco/pkg/api"
)

var (
	ErrSetupConfigUnusableSource       = errors.New("sources should not have the 'is_usable' field set to false")
	ErrSetupConfigEmptyStorageLocation = errors.New("'storageLocation' field must not be empty")
	ErrSetupConfigEmptyPath            = errors.New("'path' field must not be empty while using the 'file_association' source")
	ErrSetupConfigEmptyPathOrInput     = errors.New("'path' or 'input' fields must not be empty while using the 'input_path' or 'path_envvar' sources")
	ErrSetupConfigEmptySource          = errors.New("'source' field must not be empty")
)

func (f *Flamenco) GetVersion(e echo.Context) error {
	return e.JSON(http.StatusOK, api.FlamencoVersion{
		Version:      appinfo.ExtendedVersion(),
		Shortversion: appinfo.ApplicationVersion,
		Name:         f.config.Get().ManagerName,
		Git:          appinfo.ApplicationGitHash,
	})
}

func (f *Flamenco) GetConfiguration(e echo.Context) error {
	isFirstRun, err := f.config.Load()
	if err != nil {
		logger := requestLogger(e)
		logger.Error().Err(err).Msg("error investigating configuration")
		return sendAPIError(e, http.StatusInternalServerError, "error investigating configuration: %v", err)
	}

	return e.JSON(http.StatusOK, api.ManagerConfiguration{
		ShamanEnabled:   f.isShamanEnabled(),
		StorageLocation: f.config.EffectiveStoragePath(),
		IsFirstRun:      isFirstRun,
	})
}

func (f *Flamenco) UpdateConfigurationFile(e echo.Context) error {
	logger := requestLogger(e)

	var newConf config.Conf
	if err := e.Bind(&newConf); err != nil {
		logger.Warn().Err(err).Msg("bad request received")
		return sendAPIError(e, http.StatusBadRequest, err.Error())
	}

	// Save the final configuration to disk.
	if err := f.config.Replace(newConf); err != nil {
		logger.Error().Err(err).Msg("error saving configuration file")
		return sendAPIError(e, http.StatusInternalServerError, "setup assistant: error saving configuration file: %v", err)
	}

	return e.NoContent(http.StatusNoContent)
}

func (f *Flamenco) GetConfigurationFile(e echo.Context) error {
	config := f.config.Get()
	return e.JSON(http.StatusOK, config)
}

func (f *Flamenco) GetVariables(e echo.Context, audience api.ManagerVariableAudience, platform string) error {
	variables := f.config.ResolveVariables(
		config.VariableAudience(audience),
		config.VariablePlatform(platform),
	)

	apiVars := api.ManagerVariables{}
	for name, variable := range variables {
		apiVars[name] = api.ManagerVariable{
			IsTwoway: variable.IsTwoWay,
			Value:    variable.Value,
		}
	}

	return e.JSON(http.StatusOK, apiVars)
}

func (f *Flamenco) GetSharedStorage(e echo.Context, audience api.ManagerVariableAudience, platform string) error {
	location := f.config.EffectiveStoragePath()
	varExpand := f.config.NewVariableExpander(config.VariableAudience(audience), config.VariablePlatform(platform))

	return e.JSON(http.StatusOK, api.SharedStorageLocation{
		Audience:      audience,
		Platform:      platform,
		Location:      varExpand.Expand(location),
		ShamanEnabled: f.isShamanEnabled(),
	})
}

func (f *Flamenco) CheckSharedStoragePath(e echo.Context) error {
	logger := requestLogger(e)

	var toCheck api.CheckSharedStoragePathJSONRequestBody
	if err := e.Bind(&toCheck); err != nil {
		logger.Warn().Err(err).Msg("bad request received")
		return sendAPIError(e, http.StatusBadRequest, "invalid format")
	}

	path := toCheck.Path
	logger = logger.With().Str("path", path).Logger()
	logger.Info().Msg("checking whether this path is suitable as shared storage")

	mkError := func(cause string, args ...interface{}) error {
		if len(args) > 0 {
			cause = fmt.Sprintf(cause, args...)
		}

		logger.Warn().Str("cause", cause).Msg("shared storage path check failed")
		return e.JSON(http.StatusOK, api.PathCheckResult{
			Cause:    cause,
			IsUsable: false,
			Path:     path,
		})
	}

	// Check for emptyness.
	if path == "" {
		return mkError("An empty path is not suitable as shared storage")
	}

	// Check whether it is actually a directory.
	stat, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return mkError("This path does not exist. Choose an existing directory.")
	case err != nil:
		logger.Error().Err(err).Msg("error checking filesystem")
		return mkError("Error checking filesystem: %v", err)
	case !stat.IsDir():
		return mkError("The given path is not a directory. Choose an existing directory.")
	}

	// Check if this is the Flamenco directory itself.
	myDir, err := flamencoManagerDir()
	if err != nil {
		logger.Error().Err(err).Msg("error trying to find my own directory")
	} else if path == myDir {
		return mkError("Don't pick the installation directory of Flamenco Manager. Choose a directory dedicated to the shared storage of files.")
	}

	// See if we can create a file there.
	file, err := os.CreateTemp(path, "flamenco-writability-test-*.txt")
	if err != nil {
		return mkError("Unable to create a file in that directory: %v. "+
			"Pick an existing directory where Flamenco Manager can create files.", err)
	}

	defer func() {
		// Clean up after the test is done.
		file.Close()
		os.Remove(file.Name())
	}()

	if _, err := file.Write([]byte("Ünicöde")); err != nil {
		return mkError("unable to write to %s: %v", file.Name(), err)
	}
	if err := file.Close(); err != nil {
		// Some write errors only get reported when the file is closed, so just
		// report is as a regular write error.
		return mkError("unable to write to %s: %v", file.Name(), err)
	}

	// There is a directory, and we can create a file there. Should be good to go.
	return e.JSON(http.StatusOK, api.PathCheckResult{
		Cause:    "Directory checked successfully",
		IsUsable: true,
		Path:     path,
	})
}

func (f *Flamenco) FindBlenderExePath(e echo.Context) error {
	logger := requestLogger(e)
	ctx := e.Request().Context()

	response := api.BlenderPathFindResult{}

	// TODO: the code below is a bit too coupled with the innards of find_blender.CheckBlender().

	// Find by file association, falling back to just finding "blender" on the
	// path if not available. This uses find_blender.CheckBlender() instead of
	// find_blender.FindBlender() because the former also tries to run the found
	// executable and reports on the version of Blender.
	result, err := find_blender.CheckBlender(ctx, "")
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, exec.ErrNotFound):
		logger.Info().Msg("Blender could not be found")
	case err != nil:
		logger.Warn().AnErr("cause", err).Msg("there was an issue finding Blender")
		return sendAPIError(e, http.StatusInternalServerError, "there was an issue finding Blender: %v", err)
	default:
		response = append(response, api.BlenderPathCheckResult{
			IsUsable: true,
			Input:    result.Input,
			Path:     result.FoundLocation,
			Cause:    result.BlenderVersion,
			Source:   result.Source,
		})
	}

	if result.Source == api.BlenderPathSourceFileAssociation {
		// There could be another Blender found on $PATH.
		result, err := find_blender.CheckBlender(ctx, "blender")
		switch {
		case errors.Is(err, fs.ErrNotExist), errors.Is(err, exec.ErrNotFound):
			logger.Debug().Msg("Blender could not be found as 'blender' on $PATH")
		case err != nil:
			logger.Info().AnErr("cause", err).Msg("there was an issue finding Blender as 'blender' on $PATH")
		default:
			response = append(response, api.BlenderPathCheckResult{
				IsUsable: true,
				Input:    result.Input,
				Path:     result.FoundLocation,
				Cause:    result.BlenderVersion,
				Source:   result.Source,
			})
		}
	}

	return e.JSON(http.StatusOK, response)
}

func (f *Flamenco) CheckBlenderExePath(e echo.Context) error {
	logger := requestLogger(e)

	var toCheck api.CheckBlenderExePathJSONRequestBody
	if err := e.Bind(&toCheck); err != nil {
		logger.Warn().Err(err).Msg("bad request received")
		return sendAPIError(e, http.StatusBadRequest, "invalid format")
	}

	command := toCheck.Path
	logger = logger.With().Str("command", command).Logger()
	logger.Info().Msg("checking whether this command leads to Blender")

	ctx := e.Request().Context()
	checkResult, err := find_blender.CheckBlender(ctx, command)
	response := api.BlenderPathCheckResult{
		Input:  command,
		Source: checkResult.Source,
	}

	switch {
	case errors.Is(err, exec.ErrNotFound):
		response.Cause = "Blender could not be found"
	case err != nil:
		response.Cause = fmt.Sprintf("There was an error running the command: %v", err)
	default:
		response.IsUsable = true
		response.Path = checkResult.FoundLocation
		response.Cause = fmt.Sprintf("Found %v", checkResult.BlenderVersion)
	}

	logger.Info().
		Str("input", response.Input).
		Str("foundLocation", response.Path).
		Str("result", response.Cause).
		Bool("isUsable", response.IsUsable).
		Msg("result of command check")

	return e.JSON(http.StatusOK, response)
}

func (f *Flamenco) SaveSetupAssistantConfig(e echo.Context) error {
	logger := requestLogger(e)

	var setupAssistantCfg api.SetupAssistantConfig
	if err := e.Bind(&setupAssistantCfg); err != nil {
		logger.Warn().Err(err).Msg("setup assistant: bad request received")
		return sendAPIError(e, http.StatusBadRequest, "invalid format")
	}

	logger = logger.With().Interface("config", setupAssistantCfg).Logger()

	if err := checkSetupAssistantConfig(setupAssistantCfg); err != nil {
		logger.Error().AnErr("cause", err).Msg("setup assistant: configuration is incomplete")
		return sendAPIError(e, http.StatusBadRequest, "configuration is incomplete: %v", err)
	}

	conf := f.config.Get()
	conf.SharedStoragePath = setupAssistantCfg.StorageLocation

	var executable string
	switch setupAssistantCfg.BlenderExecutable.Source {
	case api.BlenderPathSourceFileAssociation, api.BlenderPathSourceDefault:
		// The Worker will try to use the file association when the command is set
		// to the string "blender".
		executable = "blender"
	case api.BlenderPathSourcePathEnvvar:
		// The input command can be found on $PATH, and thus we don't need to save
		// the absolute path to Blender here.
		executable = setupAssistantCfg.BlenderExecutable.Input
	case api.BlenderPathSourceInputPath, api.BlenderPathSourceSystemLocation:
		// The path should be used as-is.
		executable = setupAssistantCfg.BlenderExecutable.Path
	}
	if commandNeedsQuoting(executable) {
		executable = strconv.Quote(executable)
	}

	// own definition so that they're easier to edit later.
	conf.Variables["blender"] = config.Variable{
		Values: config.VariableValues{
			{Platform: "linux", Value: executable},
			{Platform: "windows", Value: executable},
			{Platform: "darwin", Value: executable},
		},
	}
	conf.Variables["blenderArgs"] = config.Variable{
		Values: config.VariableValues{
			{Platform: config.VariablePlatformAll, Value: config.DefaultBlenderArguments},
		},
	}

	// Save the final configuration to disk.
	if err := f.config.Save(); err != nil {
		logger.Error().Err(err).Msg("error saving configuration file")
		return sendAPIError(e, http.StatusInternalServerError, "setup assistant: error saving configuration file: %v", err)
	}

	logger.Info().Msg("setup assistant: updating configuration")

	// Request the shutdown in a goroutine, so that this one can continue sending the response.
	go f.requestRestart()

	return e.NoContent(http.StatusNoContent)
}

func (f *Flamenco) GetFarmStatus(e echo.Context) error {
	return e.JSON(http.StatusOK, f.farmstatus.Report())
}

func flamencoManagerDir() (string, error) {
	exename, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exename), nil
}

func commandNeedsQuoting(cmd string) bool {
	return strings.ContainsAny(cmd, "\n\t;()")
}

func checkSetupAssistantConfig(config api.SetupAssistantConfig) error {
	if config.StorageLocation == "" {
		return ErrSetupConfigEmptyStorageLocation
	}

	if !config.BlenderExecutable.IsUsable {
		return ErrSetupConfigUnusableSource
	}

	switch config.BlenderExecutable.Source {
	case api.BlenderPathSourceDefault:
		return nil

	case api.BlenderPathSourceFileAssociation:
		if config.BlenderExecutable.Path == "" {
			return ErrSetupConfigEmptyPath
		}

	case api.BlenderPathSourceInputPath,
		api.BlenderPathSourcePathEnvvar,
		api.BlenderPathSourceSystemLocation:
		if config.BlenderExecutable.Path == "" ||
			config.BlenderExecutable.Input == "" {
			return ErrSetupConfigEmptyPathOrInput
		}

	case "":
		return ErrSetupConfigEmptySource

	default:
		return fmt.Errorf("unknown 'source' field value: %v", config.BlenderExecutable.Source)
	}

	return nil
}
