// SPDX-License-Identifier: GPL-3.0-or-later
package persistence

import (
	"context"

	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/internal/manager/persistence/sqlc"
	"projects.blender.org/studio/flamenco/pkg/api"
)

type TaskSummary = sqlc.QueryJobTaskSummariesRow

// QueryJobTaskSummaries retrieves all tasks of the job, but not all fields of those tasks.
// Fields are synchronised with api.TaskSummary.
func (db *DB) QueryJobTaskSummaries(ctx context.Context, jobUUID string) ([]TaskSummary, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Str("job", jobUUID).Msg("querying task summaries")

	var summaries []sqlc.QueryJobTaskSummariesRow
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		summaries, err = q.QueryJobTaskSummaries(ctx, jobUUID)
		return
	})
	if err != nil {
		return nil, err
	}

	result := make([]TaskSummary, len(summaries))
	for index, task := range summaries {
		result[index] = TaskSummary(task)
	}

	return result, nil
}

// JobStatusCount is a mapping from job status to the number of jobs in that status.
type JobStatusCount map[api.JobStatus]int

func (db *DB) SummarizeJobStatuses(ctx context.Context) (JobStatusCount, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msg("database: summarizing job statuses")

	var result []sqlc.SummarizeJobStatusesRow
	err := db.queriesRO(ctx, func(q *sqlc.Queries) (err error) {
		result, err = q.SummarizeJobStatuses(ctx)
		return
	})

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
