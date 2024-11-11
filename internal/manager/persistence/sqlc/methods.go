// Code MANUALLY written to extend the SQLC structs with some extra methods.

package sqlc

import (
	"fmt"
	"strings"

	"projects.blender.org/studio/flamenco/pkg/api"
)

// SPDX-License-Identifier: GPL-3.0-or-later

func (w *Worker) Identifier() string {
	// Avoid a panic when worker.Identifier() is called on a nil pointer.
	if w == nil {
		return "-nil worker-"
	}
	return fmt.Sprintf("%s (%s)", w.Name, w.UUID)
}

// TaskTypes returns the worker's supported task types as list of strings.
func (w *Worker) TaskTypes() []string {
	return strings.Split(w.SupportedTaskTypes, ",")
}

// StatusChangeRequest stores a requested status change on the Worker.
// This just updates the Worker instance, but doesn't store the change in the
// database.
func (w *Worker) StatusChangeRequest(status api.WorkerStatus, isLazyRequest bool) {
	w.StatusRequested = status
	w.LazyStatusRequest = isLazyRequest
}

// StatusChangeClear clears the requested status change of the Worker.
// This just updates the Worker instance, but doesn't store the change in the
// database.
func (w *Worker) StatusChangeClear() {
	w.StatusRequested = ""
	w.LazyStatusRequest = false
}
