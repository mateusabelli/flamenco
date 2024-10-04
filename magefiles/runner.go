//go:build mage

package main

import (
	"context"

	"github.com/magefile/mage/sh"
	"golang.org/x/sync/errgroup"
)

// Runner allows running a group of commands sequentially, stopping at the first
// failure.
// See https://github.com/magefile/mage/issues/455 for the feature request
// to include this in Mage.
type Runner struct {
	group *errgroup.Group
	ctx   context.Context
}

// NewRunner constructs a new runner that's bound to the given context. If the
// context is done, no new command will be executed. It does NOT abort an
// already-running command.
func NewRunner(ctx context.Context) *Runner {
	group, groupctx := errgroup.WithContext(ctx)
	group.SetLimit(1)

	return &Runner{
		group: group,
		ctx:   groupctx,
	}
}

// Run the given command.
// This only runs a command if no previous command has failed yet.
func (r *Runner) Run(cmd string, args ...string) {
	r.group.Go(func() error {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		return sh.RunV(cmd, args...)
	})
}

// Wait for the commands to finish running, and return any error.
func (r *Runner) Wait() error {
	return r.group.Wait()
}
