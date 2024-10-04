//go:build mage

package main

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"projects.blender.org/studio/flamenco/internal/appinfo"
)

func packAddon(filename string) error {
	outfile, err := filepath.Abs(filename)
	if err != nil {
		return fmt.Errorf("unable make output file path absolute: %w", err)
	}

	// Open the output file.
	zipFile, err := os.Create(outfile)
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", outfile, err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	zipWriter.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})

	basePath, err := filepath.Abs("./addon") //  os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current working directory: %w", err)
	}

	// Copy all the files into the ZIP.
	addToZip := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error received from filepath.WalkDir: %w", err)
		}

		// Construct the path inside the ZIP file.
		relpath, err := filepath.Rel(basePath, path)
		if err != nil {
			return fmt.Errorf("making %s relative to %s: %w", path, basePath, err)
		}

		if d.IsDir() {
			switch {
			case filepath.Base(path) == "__pycache__":
				return fs.SkipDir
			case relpath == filepath.Join("flamenco", "manager", "docs"):
				return fs.SkipDir
			case strings.HasPrefix(filepath.Base(path), "."):
				// Skip directories like .mypy_cache, etc.
				return fs.SkipDir
			default:
				// Just recurse into this directory.
				return nil
			}
		}

		// Read the file's contents. These are just Python files and maybe a Wheel,
		// nothing huge.
		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		// Write into the ZIP file.
		fileInZip, err := zipWriter.Create(relpath)
		if err != nil {
			return fmt.Errorf("creating %s in ZIP: %w", relpath, err)
		}
		_, err = fileInZip.Write(contents)
		if err != nil {
			return fmt.Errorf("writing to %s in ZIP: %w", relpath, err)
		}

		return nil
	}

	if err := filepath.WalkDir(filepath.Join(basePath, "flamenco"), addToZip); err != nil {
		return fmt.Errorf("error filling ZIP file: %w", err)
	}

	comment := fmt.Sprintf("%s add-on for Blender, version %s",
		appinfo.ApplicationName,
		appinfo.ApplicationVersion,
	)
	if err := zipWriter.SetComment(comment); err != nil {
		return fmt.Errorf("error setting ZIP comment: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("error closing ZIP file: %w", err)
	}
	return nil
}
