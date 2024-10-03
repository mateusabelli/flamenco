//go:build !windows && !darwin

package find_blender

// SPDX-License-Identifier: GPL-3.0-or-later

const blenderExeName = "blender"

// searchDefaultPaths search any available platform-specific default locations for Blender to be in.
// Returns the path of the blender executable, or an empty string if nothing is found.
func searchDefaultPaths() string {
	return ""
}

// fileAssociation isn't implemented on non-Windows platforms.
func fileAssociation() (string, error) {
	return "", ErrNotAvailable
}
