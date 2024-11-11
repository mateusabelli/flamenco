//go:build mage

package main

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"fmt"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/magefile/mage/target"
)

const (
	goPkg = "projects.blender.org/studio/flamenco"
)

var (
	// The directory that will contain the built webapp files, and some other
	// files that will be served as static files by the Flamenco Manager web
	// server.
	webStatic = filepath.Join("web", "static")
)

// Build Flamenco Manager and Flamenco Worker, including the webapp and the add-on
func Build() {
	mg.Deps(FlamencoManager, FlamencoWorker)
}

// Build Flamenco Manager with the webapp and add-on ZIP embedded
func FlamencoManager() error {
	mg.Deps(WebappStatic)
	mg.Deps(flamencoManager)
	return nil
}

// Only build the Flamenco Manager executable, do not rebuild the webapp
func FlamencoManagerWithoutWebapp() error {
	mg.Deps(flamencoManager)
	return nil
}

func flamencoManager() error {
	return build("./cmd/flamenco-manager")
}

// Build the Flamenco Worker executable
func FlamencoWorker() error {
	return build("./cmd/flamenco-worker")
}

// Build the webapp as static files that can be served
func WebappStatic() error {
	runInstall, err := target.Dir("web/app/node_modules")
	if err != nil {
		return err
	}
	if runInstall {
		mg.SerialDeps(InstallDepsWebapp)
	}
	if err := cleanWebappStatic(); err != nil {
		return err
	}

	env := map[string]string{
		"MSYS2_ARG_CONV_EXCL": "*",
	}

	// When changing the base URL, also update the line
	// e.GET("/app/*", echo.WrapHandler(webAppHandler))
	// in `cmd/flamenco-manager/main.go`
	err = sh.RunWithV(env,
		"yarn",
		"--cwd", "web/app",
		"build",
		"--outDir", "../static",
		"--base=/app/",
		"--logLevel", "warn",
		// For debugging you can add:
		// "--minify", "false",
	)
	if err != nil {
		return err
	}

	fmt.Printf("Web app has been installed into %s\n", webStatic)

	// Build the add-on ZIP as it's part of the static web files.
	zipPath := filepath.Join(webStatic, "flamenco-addon.zip")
	return packAddon(zipPath)
}

func build(exePackage string) error {
	flags, err := buildFlags()
	if err != nil {
		return err
	}

	args := []string{"build", "-v"}
	args = append(args, flags...)
	args = append(args, exePackage)
	return sh.RunV(mg.GoCmd(), args...)
}

func buildFlags() ([]string, error) {
	hash, err := gitHash()
	if err != nil {
		return nil, err
	}

	ldflags := "" +
		fmt.Sprintf(" -X %s/internal/appinfo.ApplicationVersion=%s", goPkg, version) +
		fmt.Sprintf(" -X %s/internal/appinfo.ApplicationGitHash=%s", goPkg, hash) +
		fmt.Sprintf(" -X %s/internal/appinfo.ReleaseCycle=%s", goPkg, releaseCycle)

	flags := []string{
		"-ldflags=" + ldflags,
	}
	return flags, nil
}
