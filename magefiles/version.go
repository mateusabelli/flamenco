//go:build mage

package main

import (
	"fmt"

	"github.com/magefile/mage/sh"
)

// To update the version number in all the relevant places, update the VERSION
// variable below and run `make update-version`.
const (
	version      = "3.8-alpha2"
	releaseCycle = "alpha"
)

func gitHash() (string, error) {
	return sh.Output("git", "rev-parse", "--short", "HEAD")
}

// Show which version information would be embedded in executables
func Version() error {
	fmt.Printf("Package     : %s\n", goPkg)
	fmt.Printf("Version     : %s\n", version)

	hash, err := gitHash()
	if err != nil {
		return err
	}
	fmt.Printf("Git Hash    : %s\n", hash)
	return nil
}
