// SPDX-License-Identifier: GPL-3.0-or-later
package persistence

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/pkg/api"
)

func (db *DB) QueryJobs(ctx context.Context, apiQ api.JobsQuery) ([]*Job, error) {
	logger := log.Ctx(ctx)

	logger.Debug().Interface("q", apiQ).Msg("querying jobs")

	q := db.gormDB.WithContext(ctx).Model(&Job{})

	// WHERE
	if apiQ.StatusIn != nil {
		q = q.Where("status in ?", *apiQ.StatusIn)
	}
	if apiQ.Settings != nil {
		for setting, value := range apiQ.Settings.AdditionalProperties {
			q = q.Where("json_extract(metadata, ?) = ?", "$."+setting, value)
		}
	}
	if apiQ.Metadata != nil {
		for setting, value := range apiQ.Metadata.AdditionalProperties {
			if strings.ContainsRune(value, '%') {
				q = q.Where("json_extract(metadata, ?) like ?", "$."+setting, value)
			} else {
				q = q.Where("json_extract(metadata, ?) = ?", "$."+setting, value)
			}
		}
	}

	// OFFSET
	if apiQ.Offset != nil {
		q = q.Offset(*apiQ.Offset)
	}

	// LIMIT
	if apiQ.Limit != nil {
		q = q.Limit(*apiQ.Limit)
	}

	// ORDER BY
	if apiQ.OrderBy != nil {
		sqlOrder := ""
		for _, order := range *apiQ.OrderBy {
			if order == "" {
				continue
			}
			switch order[0] {
			case '-':
				sqlOrder = order[1:] + " desc"
			case '+':
				sqlOrder = order[1:] + " asc"
			default:
				sqlOrder = order
			}
			q = q.Order(sqlOrder)
		}
	}

	q.Preload("Tag")

	result := []*Job{}
	tx := q.Scan(&result)
	return result, tx.Error
}

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
