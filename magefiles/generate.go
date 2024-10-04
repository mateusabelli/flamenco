//go:build mage

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Generate code (OpenAPI and test mocks)
func Generate() {
	mg.Deps(GenerateGo, GeneratePy, GenerateJS)
}

// Generate Go code for Flamenco Manager and Worker
func GenerateGo(ctx context.Context) error {
	r := NewRunner(ctx)
	r.Run(mg.GoCmd(), "generate", "./pkg/api/...")
	r.Run(mg.GoCmd(), "generate", "./internal/...")
	if err := r.Wait(); err != nil {
		return err
	}

	// The generators always produce UNIX line-ends. This creates false file
	// modifications with Git. Convert them to DOS line-ends to avoid this.
	if runtime.GOOS == "windows" {
		unix2dosModifiedFiles(".gen.go$")
	}
	return nil
}

// Generate Python code for the add-on
func GeneratePy() error {
	// The generator doesn't consistently overwrite existing files, nor does it
	// remove no-longer-generated files.
	sh.Rm("addon/flamenco/manager")

	// See https://openapi-generator.tech/docs/generators/python for the options.
	err := sh.Run("java",
		"-jar", "addon/openapi-generator-cli.jar",
		"generate",
		"-i", "pkg/api/flamenco-openapi.yaml",
		"-g", "python",
		"-o", "addon/",
		"--package-name", "flamenco.manager",
		"--http-user-agent", fmt.Sprintf("Flamenco/%s (Blender add-on)", version),
		"-p", "generateSourceCodeOnly=true",
		"-p", "projectName=Flamenco",
		"-p", fmt.Sprintf("packageVersion=%s", version))
	if err != nil {
		return err
	}

	// The generator outputs files so that we can write our own tests. We don't,
	// though, so it's better to just remove those placeholders.
	sh.Rm("addon/flamenco/manager/test")

	// The generators always produce UNIX line-ends. This creates false file
	// modifications with Git. Convert them to DOS line-ends to avoid this.
	if runtime.GOOS == "windows" {
		unix2dosModifiedFiles("addon/flamenco/manager")
	}

	return nil
}

// Generate JavaScript code for the webapp
func GenerateJS() error {
	const (
		jsOutDir  = "web/app/src/manager-api"
		jsTempDir = "web/_tmp-manager-api-javascript"
	)
	sh.Rm(jsOutDir)
	sh.Rm(jsTempDir)

	// See https://openapi-generator.tech/docs/generators/javascript for the options.
	// Version '0.0.0' is used as NPM doesn't like Git hashes as versions.
	//
	// -p modelPropertyNaming=original is needed because otherwise the generator will
	// use original naming internally, but generate docs with camelCase, and then
	// things don't work properly.
	err := sh.Run("java",
		"-jar", "addon/openapi-generator-cli.jar",
		"generate",
		"-i", "pkg/api/flamenco-openapi.yaml",
		"-g", "javascript",
		"-o", jsTempDir,
		"--http-user-agent", fmt.Sprintf("Flamenco/%s / webbrowser", version),
		"-p", "projectName=flamenco-manager",
		"-p", "projectVersion=0.0.0",
		"-p", "apiPackage=manager",
		"-p", "disallowAdditionalPropertiesIfNotPresent=false",
		"-p", "usePromises=true",
		"-p", "moduleName=flamencoManager")
	if err != nil {
		return err
	}

	// Cherry-pick the generated sources, and remove everything else.
	if err := os.Rename(filepath.Join(jsTempDir, "src"), jsOutDir); err != nil {
		return err
	}
	sh.Rm(jsTempDir)

	if runtime.GOOS == "windows" {
		unix2dosModifiedFiles(jsOutDir)
	}

	return nil
}

// unix2dosModifiedFiles changes line ends in files Git considers modified that match the given pattern.
func unix2dosModifiedFiles(pattern string) {
	// Get modified files from Git. Expected lines like:
	//
	// 	M pkg/api/openapi_client.gen.go
	// 	M pkg/api/openapi_server.gen.go
	// 	M pkg/api/openapi_spec.gen.go
	// 	M pkg/api/openapi_types.gen.go
	// ?? magefiles/generate.go

	gitStatus, err := sh.Output("git", "status", "--porcelain")
	if err != nil {
		panic(fmt.Sprintf("error running 'git status': %s", err))
	}

	// Construct a list of modified files that match the pattern.
	patternRe := regexp.MustCompile(pattern)
	modified := []string{}
	for _, line := range strings.Split(gitStatus, "\n") {
		if line[0:3] != " M " {
			continue
		}
		if !patternRe.MatchString(line[3:]) {
			continue
		}
		modified = append(modified, line[3:])
	}

	// Run unix2dos on all found files.
	for _, path := range modified {
		unix2dos(path)
	}
}

func unix2dos(filename string) {
	if mg.Verbose() {
		fmt.Printf("unix2dos %s\n", filename)
	}

	// TODO: rewrite to stream the data instead of reading, copying, and writing
	// everything.
	contents, err := os.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprintf("error converting UNIX to DOS line ends: %v", err))
	}

	lines := bytes.Split(contents, []byte("\n"))
	err = os.WriteFile(filename, bytes.Join(lines, []byte("\r\n")), os.ModePerm)
	if err != nil {
		panic(fmt.Sprintf("error writing DOS line ends: %v", err))
	}
}
