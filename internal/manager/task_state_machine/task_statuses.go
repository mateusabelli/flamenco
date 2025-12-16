package task_state_machine

// SPDX-License-Identifier: GPL-3.0-or-later

import "projects.blender.org/studio/flamenco/pkg/api"

var (
	// Workers are allowed to keep running tasks when they are in this status.
	// 'queued', 'claimed-by-manager', and 'soft-failed' aren't considered runnable,
	// as those statuses indicate the task wasn't assigned to a Worker by the scheduler.
	runnableStatuses = map[api.TaskStatus]bool{
		api.TaskStatusActive: true,
	}
)

// CanWorkerRun returns whether the given status is considered "runnable".
// In other words, workers are allowed to keep running such tasks.
func CanWorkerRun(status api.TaskStatus) bool {
	return runnableStatuses[status]
}
