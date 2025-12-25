package moremock

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"time"

	"go.uber.org/mock/gomock"
)

// ContextWithDeadline returns a gomock matcher to match a context.Context()
// with a deadline in the future.
func ContextWithDeadline() gomock.Matcher { return ctxWithDeadlineMatcher{} }

type ctxWithDeadlineMatcher struct{}

// Matches returns whether x is a match.
func (m ctxWithDeadlineMatcher) Matches(x interface{}) bool {
	ctx, ok := x.(context.Context)
	if !ok {
		return false
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		return false
	}

	return time.Now().Before(deadline)
}

// String describes what the matcher matches.
func (m ctxWithDeadlineMatcher) String() string {
	return "is context with deadline in the future"
}
