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

	var result []*Task
	tx := db.gormDB.WithContext(ctx).Model(&Task{}).
		Select("tasks.id", "tasks.uuid", "tasks.name", "tasks.priority", "tasks.status", "tasks.type", "tasks.updated_at").
		Joins("left join jobs on jobs.id = tasks.job_id").
		Where("jobs.uuid=?", jobUUID).
		Scan(&result)

	return result, tx.Error
}

// JobStatusCount is a mapping from job status to the number of jobs in that status.
type JobStatusCount map[api.JobStatus]int

func (db *DB) SummarizeJobStatuses(ctx context.Context) (JobStatusCount, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msg("database: summarizing job statuses")

	// Query the database using a data structure that's easy to handle in GORM.
	type queryResult struct {
		Status      api.JobStatus
		StatusCount int
	}
	result := []*queryResult{}
	tx := db.gormDB.WithContext(ctx).Model(&Job{}).
		Select("status as Status", "count(id) as StatusCount").
		Group("status").
		Scan(&result)
	if tx.Error != nil {
		return nil, jobError(tx.Error, "summarizing job statuses")
	}

	// Convert the array-of-structs to a map that's easier to handle by the caller.
	statusCounts := make(JobStatusCount)
	for _, singleStatusCount := range result {
		statusCounts[singleStatusCount.Status] = singleStatusCount.StatusCount
	}

	return statusCounts, nil
}
