// SPDX-License-Identifier: GPL-3.0-or-later
package persistence

import (
	"context"

	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// QueryJobTaskSummaries retrieves all tasks of the job, but not all fields of those tasks.
// Fields are synchronised with api.TaskSummary.
func (db *DB) QueryJobTaskSummaries(ctx context.Context, jobUUID string) ([]*Task, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Str("job", jobUUID).Msg("querying task summaries")

	queries := db.queries()
	sqlcPartialTasks, err := queries.QueryJobTaskSummaries(ctx, jobUUID)
	if err != nil {
		return nil, err
	}

	// Convert to partial GORM tasks.
	gormTasks := make([]*Task, len(sqlcPartialTasks))
	for index, task := range sqlcPartialTasks {
		gormTask := Task{
			Model: Model{
				ID:        uint(task.ID),
				UpdatedAt: task.UpdatedAt.Time,
			},

			UUID:       task.UUID,
			Name:       task.Name,
			Type:       task.Type,
			IndexInJob: int(task.IndexInJob),
			Priority:   int(task.Priority),
			Status:     api.TaskStatus(task.Status),
			JobUUID:    jobUUID,
		}
		gormTasks[index] = &gormTask
	}

	return gormTasks, nil
}

// JobStatusCount is a mapping from job status to the number of jobs in that status.
type JobStatusCount map[api.JobStatus]int

func (db *DB) SummarizeJobStatuses(ctx context.Context) (JobStatusCount, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msg("database: summarizing job statuses")

	queries := db.queries()
	result, err := queries.SummarizeJobStatuses(ctx)
	if err != nil {
		return nil, jobError(err, "summarizing job statuses")
	}

	// Convert the array-of-structs to a map that's easier to handle by the caller.
	statusCounts := make(JobStatusCount)
	for _, singleStatusCount := range result {
		statusCounts[api.JobStatus(singleStatusCount.Status)] = int(singleStatusCount.StatusCount)
	}

	return statusCounts, nil
}
