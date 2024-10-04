//go:build mage

package main

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var (
	generators = []string{
		"github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.9.0",
		"github.com/golang/mock/mockgen@v1.6.0",
	}
)

// Install build-time dependencies: code generators and NodeJS dependencies.
func InstallDeps() {
	mg.SerialDeps(InstallGenerators, InstallDepsWebapp)
}

// Install code generators.
func InstallGenerators(ctx context.Context) error {
	r := NewRunner(ctx)
	for _, pkg := range generators {
		r.Run(mg.GoCmd(), "install", pkg)
	}
	return r.Wait()
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
