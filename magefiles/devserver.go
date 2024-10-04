//go:build mage

package main

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"github.com/magefile/mage/sh"
)

func DevServerWebapp() error {
	return sh.RunV("yarn", "--cwd", "web/app", "run", "dev", "--host")
}
