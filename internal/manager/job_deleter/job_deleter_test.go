package job_deleter

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"projects.blender.org/studio/flamenco/internal/manager/job_deleter/mocks"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/shaman"
)

type JobDeleterMocks struct {
	persist     *mocks.MockPersistenceService
	storage     *mocks.MockStorage
	broadcaster *mocks.MockChangeBroadcaster
	shaman      *mocks.MockShaman

	ctx    context.Context
	cancel context.CancelFunc
}

func TestQueueJobDeletion(t *testing.T) {
	s, finish, mocks := jobDeleterTestFixtures(t)
	defer finish()

	mocks.broadcaster.EXPECT().BroadcastJobUpdate(gomock.Any()).Times(3)

	job1 := &persistence.Job{UUID: "2f7d910f-08a6-4b0f-8ecb-b3946939ed1b"}
	mocks.persist.EXPECT().RequestJobDeletion(mocks.ctx, job1)
	require.NoError(t, s.QueueJobDeletion(mocks.ctx, job1))

	// Call twice more to overflow the queue.
	job2 := &persistence.Job{UUID: "e8fbe41c-ed24-46df-ba63-8d4f5524071b"}
	mocks.persist.EXPECT().RequestJobDeletion(mocks.ctx, job2)
	require.NoError(t, s.QueueJobDeletion(mocks.ctx, job2))

	job3 := &persistence.Job{UUID: "deeab6ba-02cd-42c0-b7bc-2367a2f04c7d"}
	mocks.persist.EXPECT().RequestJobDeletion(mocks.ctx, job3)
	require.NoError(t, s.QueueJobDeletion(mocks.ctx, job3))

	if assert.Len(t, s.queue, 2, "the first two job UUID should be queued") {
		assert.Equal(t, job1.UUID, <-s.queue)
		assert.Equal(t, job2.UUID, <-s.queue)
	}
}

func TestQueuePendingDeletions(t *testing.T) {
	s, finish, mocks := jobDeleterTestFixtures(t)
	defer finish()

	// Queue one more job than fits.
	job1 := "aa420164-926a-45d5-ae8b-510ff3d2cd4d"
	job2 := "e5feadee-999e-48c2-853d-9db94e7623b0"
	job3 := "8516ac60-787c-411e-80a7-026456034da4"

	mocks.persist.EXPECT().
		FetchJobsDeletionRequested(mocks.ctx).
		Return([]string{job1, job2, job3}, nil)
	s.queuePendingDeletions(mocks.ctx)
	if assert.Len(t, s.queue, 2, "the first two job UUIDs should be queued") {
		assert.Equal(t, job1, <-s.queue)
		assert.Equal(t, job2, <-s.queue)
	}
}

func TestQueuePendingDeletionsUnhappy(t *testing.T) {
	s, finish, mocks := jobDeleterTestFixtures(t)
	defer finish()

	// Any error fetching the deletion-requested jobs should just be logged, and
	// not cause any issues.
	mocks.persist.EXPECT().
		FetchJobsDeletionRequested(mocks.ctx).
		Return(nil, errors.New("mocked DB failure"))

	s.queuePendingDeletions(mocks.ctx)
	assert.Len(t, s.queue, 0)
}

func TestDeleteJobWithoutShaman(t *testing.T) {
	s, finish, mocks := jobDeleterTestFixtures(t)
	defer finish()

	jobUUID := "2f7d910f-08a6-4b0f-8ecb-b3946939ed1b"

	mocks.shaman.EXPECT().IsEnabled().Return(false).AnyTimes()
	mocks.persist.EXPECT().
		FetchJobsDeletionRequested(mocks.ctx).
		Return([]string{jobUUID}, nil).
		AnyTimes()

	// Mock log storage deletion failure. This should prevent the deletion from the database.
	mocks.storage.EXPECT().
		RemoveJobStorage(mocks.ctx, jobUUID).
		Return(errors.New("intended log file deletion failure"))
	assert.Error(t, s.deleteJob(mocks.ctx, jobUUID))

	// Mock that log storage deletion is ok, but database is not.
	mocks.storage.EXPECT().RemoveJobStorage(mocks.ctx, jobUUID)
	mocks.persist.EXPECT().DeleteJob(mocks.ctx, jobUUID).
		Return(errors.New("mocked DB error"))
	assert.Error(t, s.deleteJob(mocks.ctx, jobUUID))

	// Mock that everything went OK.
	mocks.storage.EXPECT().RemoveJobStorage(mocks.ctx, jobUUID)
	mocks.persist.EXPECT().DeleteJob(mocks.ctx, jobUUID)
	mocks.persist.EXPECT().RequestIntegrityCheck()
	mocks.broadcaster.EXPECT().BroadcastJobUpdate(gomock.Any())
	require.NoError(t, s.deleteJob(mocks.ctx, jobUUID))
}

func TestDeleteJobWithShaman(t *testing.T) {
	s, finish, mocks := jobDeleterTestFixtures(t)
	defer finish()

	jobUUID := "2f7d910f-08a6-4b0f-8ecb-b3946939ed1b"

	mocks.shaman.EXPECT().IsEnabled().Return(true).AnyTimes()
	mocks.persist.EXPECT().
		FetchJobsDeletionRequested(mocks.ctx).
		Return([]string{jobUUID}, nil).
		AnyTimes()

	shamanCheckoutID := "010_0431_lighting"
	dbJob := persistence.Job{
		UUID: jobUUID,
		Name: "сцена/shot/010_0431_lighting",
		Storage: persistence.JobStorageInfo{
			ShamanCheckoutID: shamanCheckoutID,
		},
	}
	mocks.persist.EXPECT().FetchJob(mocks.ctx, jobUUID).Return(&dbJob, nil).AnyTimes()

	// Mock that Shaman deletion failed. The rest of the deletion should be
	// blocked by this.
	mocks.shaman.EXPECT().EraseCheckout(shamanCheckoutID).Return(errors.New("mocked failure"))
	assert.Error(t, s.deleteJob(mocks.ctx, jobUUID))

	// Mock that Shaman deletion couldn't happen because the checkout dir doesn't
	// exist. The rest of the deletion should continue.
	mocks.shaman.EXPECT().EraseCheckout(shamanCheckoutID).Return(shaman.ErrDoesNotExist)
	// Mock log storage deletion failure. This should prevent the deletion from the database.
	mocks.storage.EXPECT().
		RemoveJobStorage(mocks.ctx, jobUUID).
		Return(errors.New("intended log file deletion failure"))
	assert.Error(t, s.deleteJob(mocks.ctx, jobUUID))

	// Mock that log storage deletion is ok, but database is not.
	mocks.shaman.EXPECT().EraseCheckout(shamanCheckoutID)
	mocks.storage.EXPECT().RemoveJobStorage(mocks.ctx, jobUUID)
	mocks.persist.EXPECT().DeleteJob(mocks.ctx, jobUUID).
		Return(errors.New("mocked DB error"))
	assert.Error(t, s.deleteJob(mocks.ctx, jobUUID))

	// Mock that everything went OK.
	mocks.shaman.EXPECT().EraseCheckout(shamanCheckoutID)
	mocks.storage.EXPECT().RemoveJobStorage(mocks.ctx, jobUUID)
	mocks.persist.EXPECT().DeleteJob(mocks.ctx, jobUUID)
	mocks.persist.EXPECT().RequestIntegrityCheck()
	mocks.broadcaster.EXPECT().BroadcastJobUpdate(gomock.Any())
	require.NoError(t, s.deleteJob(mocks.ctx, jobUUID))
}

func jobDeleterTestFixtures(t *testing.T) (*Service, func(), *JobDeleterMocks) {
	mockCtrl := gomock.NewController(t)

	mocks := &JobDeleterMocks{
		persist:     mocks.NewMockPersistenceService(mockCtrl),
		storage:     mocks.NewMockStorage(mockCtrl),
		broadcaster: mocks.NewMockChangeBroadcaster(mockCtrl),
		shaman:      mocks.NewMockShaman(mockCtrl),
	}

	ctx, cancel := context.WithCancel(context.Background())
	mocks.ctx = ctx
	mocks.cancel = cancel

	// This should be called at the end of each unit test.
	finish := func() {
		mocks.cancel()
		jobDeletionQueueSize = defaultJobDeletionQueueSize
	}
	jobDeletionQueueSize = 2

	s := NewService(
		mocks.persist,
		mocks.storage,
		mocks.broadcaster,
		mocks.shaman,
	)
	return s, finish, mocks
}
