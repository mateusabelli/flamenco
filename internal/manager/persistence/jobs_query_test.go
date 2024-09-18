// Package persistence provides the database interface for Flamenco Manager.
package persistence

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projects.blender.org/studio/flamenco/internal/manager/job_compilers"
	"projects.blender.org/studio/flamenco/internal/uuid"
	"projects.blender.org/studio/flamenco/pkg/api"
)

func TestQueryJobTaskSummaries(t *testing.T) {
	ctx, close, db, job, authoredJob := jobTasksTestFixtures(t)
	defer close()

	expectTaskUUIDs := map[string]bool{}
	for _, task := range authoredJob.Tasks {
		expectTaskUUIDs[task.UUID] = true
	}

	// Create another test job, just to check we get the right tasks back.
	otherAuthoredJob := createTestAuthoredJobWithTasks()
	otherAuthoredJob.Status = api.JobStatusActive
	for i := range otherAuthoredJob.Tasks {
		otherAuthoredJob.Tasks[i].UUID = uuid.New()
		otherAuthoredJob.Tasks[i].Dependencies = []*job_compilers.AuthoredTask{}
	}
	otherAuthoredJob.JobID = "138678c8-efd0-452b-ac05-397ff4c02b26"
	otherAuthoredJob.Metadata["project"] = "Other Project"
	persistAuthoredJob(t, ctx, db, otherAuthoredJob)

	// Sanity check for the above code, there should be 6 tasks overall, 3 per job.
	var numTasks int64
	tx := db.gormDB.Model(&Task{}).Count(&numTasks)
	require.NoError(t, tx.Error)
	assert.Equal(t, int64(6), numTasks)

	// Get the task summaries of a particular job.
	summaries, err := db.QueryJobTaskSummaries(ctx, job.UUID)
	require.NoError(t, err)

	assert.Len(t, summaries, len(expectTaskUUIDs))
	for _, summary := range summaries {
		assert.True(t, expectTaskUUIDs[summary.UUID], "%q should be in %v", summary.UUID, expectTaskUUIDs)
	}
}

func TestSummarizeJobStatuses(t *testing.T) {
	ctx, close, db, job1, authoredJob1 := jobTasksTestFixtures(t)
	defer close()

	// Create another job
	authoredJob2 := duplicateJobAndTasks(authoredJob1)
	job2 := persistAuthoredJob(t, ctx, db, authoredJob2)

	// Test the summary.
	summary, err := db.SummarizeJobStatuses(ctx)
	require.NoError(t, err)
	assert.Equal(t, JobStatusCount{api.JobStatusUnderConstruction: 2}, summary)

	// Change the jobs so that each has a unique status.
	job1.Status = api.JobStatusQueued
	require.NoError(t, db.SaveJobStatus(ctx, job1))
	job2.Status = api.JobStatusFailed
	require.NoError(t, db.SaveJobStatus(ctx, job2))

	// Test the summary.
	summary, err = db.SummarizeJobStatuses(ctx)
	require.NoError(t, err)
	assert.Equal(t, JobStatusCount{
		api.JobStatusQueued: 1,
		api.JobStatusFailed: 1,
	}, summary)

	// Delete all jobs.
	require.NoError(t, db.DeleteJob(ctx, job1.UUID))
	require.NoError(t, db.DeleteJob(ctx, job2.UUID))

	// Test the summary.
	summary, err = db.SummarizeJobStatuses(ctx)
	require.NoError(t, err)
	assert.Equal(t, JobStatusCount{}, summary)
}

// Check that a context timeout can be detected by inspecting the
// returned error.
func TestSummarizeJobStatusesTimeout(t *testing.T) {
	ctx, close, db, _, _ := jobTasksTestFixtures(t)
	defer close()

	subCtx, subCtxCancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer subCtxCancel()

	// Force a timeout of the context. And yes, even when a nanosecond is quite
	// short, it is still necessary to wait.
	time.Sleep(2 * time.Millisecond)

	summary, err := db.SummarizeJobStatuses(subCtx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, summary)
}
