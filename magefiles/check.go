//go:build mage

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"golang.org/x/vuln/scan"
	"honnef.co/go/tools/lintcmd"
	lintcmdversion "honnef.co/go/tools/lintcmd/version"
	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"
	"honnef.co/go/tools/unused"
)

// Run unit tests, check for vulnerabilities, and run the linter
func Check(ctx context.Context) {
	mg.CtxDeps(ctx, Test, Govulncheck, Staticcheck, Vet)
}

// Run unit tests
func Test(ctx context.Context) error {
	return sh.RunV(mg.GoCmd(), "test", "-short", "-failfast", "./...")
}

// Check for known vulnerabilities.
func Govulncheck(ctx context.Context) error {
	cmd := scan.Command(ctx, "./...")
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

// Analyse the source code.
func Staticcheck() error {
	cmd := lintcmd.NewCommand("staticcheck")
	cmd.SetVersion(lintcmdversion.Version, lintcmdversion.MachineVersion)
	cmd.ParseFlags([]string{"./..."})
	cmd.AddAnalyzers(simple.Analyzers...)
	cmd.AddAnalyzers(staticcheck.Analyzers...)
	cmd.AddAnalyzers(stylecheck.Analyzers...)
	cmd.AddAnalyzers(unused.Analyzer)

	exitCode := cmd.Execute()
	if exitCode != 0 {
		return errors.New("staticcheck failed")
	}
	fmt.Println("staticcheck ok")
	return nil
}

// Run `go vet`
func Vet() error {
	return sh.RunV(mg.GoCmd(), "vet", "./...")
}
