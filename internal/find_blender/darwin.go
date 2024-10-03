//go:build darwin

package find_blender

import (
	"os"

	"github.com/rs/zerolog/log"
)

// SPDX-License-Identifier: GPL-3.0-or-later

const (
	blenderExeName = "Blender"
	defaultPath    = "/Applications/Blender.app/Contents/MacOS/Blender"
)

// fileAssociation isn't implemented on non-Windows platforms.
func fileAssociation() (string, error) {
	return "", ErrNotAvailable
}

// searchDefaultPaths search any available platform-specific default locations for Blender to be in.
// Returns the path of the blender executable, or an empty string if nothing is found.
func searchDefaultPaths() string {
	stat, err := os.Stat(defaultPath)

	switch {
	case os.IsNotExist(err):
		return ""
	case err != nil:
		log.Warn().
			AnErr("cause", err).
			Str("path", defaultPath).
			Msg("could not check default path, ignoring")
		return ""
	case stat.IsDir():
		log.Warn().
			Str("path", defaultPath).
			Msg("expected Blender executable, but is a directory, ignoring")
		return ""
	}

	return defaultPath
}
