//go:build mage

package main

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"github.com/magefile/mage/sh"
)

// Install build-time dependencies: NodeJS dependencies.
func InstallDeps() {
	InstallDepsWebapp()
}

// Use Yarn to install the webapp's NodeJS dependencies
func InstallDepsWebapp() error {
	env := map[string]string{
		"MSYS2_ARG_CONV_EXCL": "*",
	}
	return sh.RunWithV(env,
		"yarn",
		"--cwd", "web/app",
		"install",
	)
}
