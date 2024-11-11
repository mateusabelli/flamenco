//go:build tools

// This file is a bit of a hacky workaround a limitation of `go mod tidy`. It
// will never be built, but `go mod tidy` will see the packages imported here as
// dependencies of the Flamenco project, and not remove them from `go.mod`.

package main

import (
	// Go code generators:
	_ "github.com/deepmap/oapi-codegen/cmd/oapi-codegen"
	_ "github.com/golang/mock/mockgen"

	// Our build tool. Normally this isn't necessary, but it's needed to be able
	// to build the tool when `go mod vendor` has been used to vendor the
	// dependencies.
	_ "github.com/magefile/mage/mage"
)
