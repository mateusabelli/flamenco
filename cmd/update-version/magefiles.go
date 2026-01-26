package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

const mageFile = "magefiles/version.go"

// updateMagefiles changes the version number in the Mage files.
// Returns whether the file actually changed.
func updateMagefiles() bool {
	logger := log.With().Str("filename", mageFile).Logger()

	// Parse the mage file as AST.
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, mageFile, nil, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		logger.Fatal().Err(err).Msgf("could not update mage version file")
		return false
	}

	// Perform replacements on the AST.
	replacements := map[string]string{
		"version":      cliArgs.newVersion,
		"releaseCycle": cliArgs.releaseCycle,
	}

	var (
		lastIdent       *ast.Ident // Last-seen identifier.
		anyFieldChanged bool
	)

	ast.Inspect(astFile, func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.Ident:
			lastIdent = x

		case *ast.BasicLit:
			replacement, ok := replacements[lastIdent.Name]
			if ok {
				newValue := fmt.Sprintf("%q", replacement)
				if x.Value != newValue {
					logger.Info().
						Str("old", x.Value).
						Str("new", newValue).
						Msg("updating mage version file")
					x.Value = newValue
					anyFieldChanged = true
				}
			}
		}

		return true
	})

	// Open a temporary file for writing.
	mageDir := filepath.Dir(mageFile)
	writer, err := os.CreateTemp(mageDir, filepath.Base(mageFile)+"*.go")
	if err != nil {
		log.Fatal().Err(err).Msgf("cannot create file in %s", mageDir)
	}
	defer func() {
		if err := writer.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			log.Fatal().Err(err).Str("file", writer.Name()).Msg("closing file")
		}
	}()

	// Write the altered AST to the temp file.
	if err := format.Node(writer, fset, astFile); err != nil {
		log.Fatal().Err(err).Msgf("cannot write updated version of %s to %s", mageFile, writer.Name())
	}

	// Close the file.
	if err := writer.Close(); err != nil {
		log.Fatal().Err(err).Msgf("cannot close %s", writer.Name())
	}

	// Overwrite the original mage file with the temp file.
	if err := os.Rename(writer.Name(), mageFile); err != nil {
		log.Fatal().Err(err).Msgf("cannot rename %s to %s", writer.Name(), mageFile)
	}

	return anyFieldChanged
}
