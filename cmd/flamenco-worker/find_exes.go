package main

import (
	"context"
	"errors"
	"io/fs"
	"os/exec"
	"time"

	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/find_blender"
	"projects.blender.org/studio/flamenco/internal/find_ffmpeg"
	"projects.blender.org/studio/flamenco/pkg/website"
)

// findFFmpeg tries to find FFmpeg, in order to show its version (if found) or a warning (if not).
func findFFmpeg() {
	result, err := find_ffmpeg.Find()
	switch {
	case errors.Is(err, fs.ErrNotExist):
		log.Warn().Msg("FFmpeg could not be found on this system, render jobs may not run correctly")
	case err != nil:
		log.Warn().Err(err).Msg("there was an unexpected error finding FFmpeg on this system, render jobs may not run correctly")
	default:
		log.Info().Str("path", result.Path).Str("version", result.Version).Msg("FFmpeg found on this system")
	}
}

// findBlender tries to find Blender, in order to show its version (if found) or a message (if not).
func findBlender() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	helpMsg := "Flamenco Manager will have to supply the full path to Blender when tasks are sent " +
		"to this Worker. For more info see " + website.DocVariablesURL

	result, err := find_blender.Find(ctx)
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, exec.ErrNotFound):
		log.Warn().Msg("Blender could not be found. " + helpMsg)
	case err != nil:
		log.Warn().AnErr("cause", err).Msg("There was an error finding Blender on this system. " + helpMsg)
	default:
		log.Info().
			Str("path", result.FoundLocation).
			Str("version", result.BlenderVersion).
			Msg("Blender found on this system, it will be used unless the Flamenco Manager configuration specifies a different path.")
	}
}
