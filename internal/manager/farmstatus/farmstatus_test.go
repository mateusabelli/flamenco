// package farmstatus provides a status indicator for the entire Flamenco farm.
package farmstatus

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"projects.blender.org/studio/flamenco/internal/manager/farmstatus/mocks"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

type Fixtures struct {
	service  *Service
	persist  *mocks.MockPersistenceService
	eventbus *mocks.MockEventBus
	ctx      context.Context
}

func TestFarmStatusStarting(t *testing.T) {
	f := fixtures(t)
	report := f.service.Report()
	assert.Equal(t, api.FarmStatusStarting, report.Status)
}

func TestFarmStatusLoop(t *testing.T) {
	f := fixtures(t)

	// Mock an "active" status.
	f.mockWorkerStatuses(persistence.WorkerStatusCount{
		api.WorkerStatusOffline: 2,
		api.WorkerStatusAsleep:  1,
		api.WorkerStatusError:   1,
		api.WorkerStatusAwake:   3,
	})
	f.mockJobStatuses(persistence.JobStatusCount{
		api.JobStatusActive: 1,
	})

	// Before polling, the status should still be 'starting'.
	report := f.service.Report()
	assert.Equal(t, api.FarmStatusStarting, report.Status)

	// After a single poll, the report should have been updated.
	f.eventbus.EXPECT().BroadcastFarmStatusEvent(api.EventFarmStatus{Status: api.FarmStatusActive})
	f.service.poll(f.ctx)
	report = f.service.Report()
	assert.Equal(t, api.FarmStatusActive, report.Status)
}

func TestCheckFarmStatusInoperative(t *testing.T) {
	f := fixtures(t)

	// "inoperative": no workers.
	f.mockWorkerStatuses(persistence.WorkerStatusCount{})
	report := f.service.checkFarmStatus(f.ctx)
	require.NotNil(t, report)
	assert.Equal(t, api.FarmStatusInoperative, report.Status)

	// "inoperative": all workers offline.
	f.mockWorkerStatuses(persistence.WorkerStatusCount{
		api.WorkerStatusOffline: 3,
	})
	report = f.service.checkFarmStatus(f.ctx)
	require.NotNil(t, report)
	assert.Equal(t, api.FarmStatusInoperative, report.Status)

	// "inoperative": some workers offline, some in error,
	f.mockWorkerStatuses(persistence.WorkerStatusCount{
		api.WorkerStatusOffline: 2,
		api.WorkerStatusError:   1,
	})
	report = f.service.checkFarmStatus(f.ctx)
	require.NotNil(t, report)
	assert.Equal(t, api.FarmStatusInoperative, report.Status)
}

func TestCheckFarmStatusActive(t *testing.T) {
	f := fixtures(t)

	// "active" # Actively working on jobs.
	f.mockWorkerStatuses(persistence.WorkerStatusCount{
		api.WorkerStatusOffline: 2,
		api.WorkerStatusAsleep:  1,
		api.WorkerStatusError:   1,
		api.WorkerStatusAwake:   3,
	})
	f.mockJobStatuses(persistence.JobStatusCount{
		api.JobStatusActive: 1,
	})
	report := f.service.checkFarmStatus(f.ctx)
	require.NotNil(t, report)
	assert.Equal(t, api.FarmStatusActive, report.Status)
}

func TestCheckFarmStatusWaiting(t *testing.T) {
	f := fixtures(t)

	// "waiting": Active job, and only sleeping workers.
	f.mockWorkerStatuses(persistence.WorkerStatusCount{
		api.WorkerStatusAsleep: 1,
	})
	f.mockJobStatuses(persistence.JobStatusCount{
		api.JobStatusActive: 1,
	})
	report := f.service.checkFarmStatus(f.ctx)
	require.NotNil(t, report)
	assert.Equal(t, api.FarmStatusWaiting, report.Status)

	// "waiting": Queued job, and awake worker. It could pick up the job any
	// second now, but it could also have been blocklisted already.
	f.mockWorkerStatuses(persistence.WorkerStatusCount{
		api.WorkerStatusAsleep: 1,
		api.WorkerStatusAwake:  1,
	})
	f.mockJobStatuses(persistence.JobStatusCount{
		api.JobStatusQueued: 1,
	})
	report = f.service.checkFarmStatus(f.ctx)
	require.NotNil(t, report)
	assert.Equal(t, api.FarmStatusWaiting, report.Status)
}

func TestCheckFarmStatusIdle(t *testing.T) {
	f := fixtures(t)

	// "idle" # Farm could be active, but has no work to do.
	f.mockWorkerStatuses(persistence.WorkerStatusCount{
		api.WorkerStatusOffline: 2,
		api.WorkerStatusAsleep:  1,
		api.WorkerStatusAwake:   1,
	})
	f.mockJobStatuses(persistence.JobStatusCount{
		api.JobStatusCompleted:       1,
		api.JobStatusCancelRequested: 1,
	})
	report := f.service.checkFarmStatus(f.ctx)
	require.NotNil(t, report)
	assert.Equal(t, api.FarmStatusIdle, report.Status)
}

func TestCheckFarmStatusAsleep(t *testing.T) {
	f := fixtures(t)

	// "asleep": No worker is awake, some are asleep, no work to do.
	f.mockWorkerStatuses(persistence.WorkerStatusCount{
		api.WorkerStatusOffline: 2,
		api.WorkerStatusAsleep:  2,
	})
	f.mockJobStatuses(persistence.JobStatusCount{
		api.JobStatusCanceled:  10,
		api.JobStatusCompleted: 4,
		api.JobStatusFailed:    2,
	})
	report := f.service.checkFarmStatus(f.ctx)
	require.NotNil(t, report)
	assert.Equal(t, api.FarmStatusAsleep, report.Status)
}

func TestFarmStatusEvent(t *testing.T) {
	f := fixtures(t)

	// "inoperative": no workers.
	f.mockWorkerStatuses(persistence.WorkerStatusCount{})
	f.eventbus.EXPECT().BroadcastFarmStatusEvent(api.EventFarmStatus{
		Status: api.FarmStatusInoperative,
	})
	f.service.poll(f.ctx)

	// Re-polling should not trigger any event, as the status doesn't change.
	f.mockWorkerStatuses(persistence.WorkerStatusCount{})
	f.service.poll(f.ctx)

	// "active": Actively working on jobs.
	f.mockWorkerStatuses(persistence.WorkerStatusCount{api.WorkerStatusAwake: 3})
	f.mockJobStatuses(persistence.JobStatusCount{api.JobStatusActive: 1})
	f.eventbus.EXPECT().BroadcastFarmStatusEvent(api.EventFarmStatus{
		Status: api.FarmStatusActive,
	})
	f.service.poll(f.ctx)
}

func Test_allIn(t *testing.T) {
	type args struct {
		statuses   map[api.WorkerStatus]int
		shouldBeIn []api.WorkerStatus
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"none", args{map[api.WorkerStatus]int{}, []api.WorkerStatus{api.WorkerStatusAsleep}}, true},
		{"match-only", args{
			map[api.WorkerStatus]int{api.WorkerStatusAsleep: 5},
			[]api.WorkerStatus{api.WorkerStatusAsleep},
		}, true},
		{"match-some", args{
			map[api.WorkerStatus]int{api.WorkerStatusAsleep: 5, api.WorkerStatusOffline: 2},
			[]api.WorkerStatus{api.WorkerStatusAsleep},
		}, false},
		{"match-all", args{
			map[api.WorkerStatus]int{api.WorkerStatusAsleep: 5, api.WorkerStatusOffline: 2},
			[]api.WorkerStatus{api.WorkerStatusAsleep, api.WorkerStatusOffline},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allIn(tt.args.statuses, tt.args.shouldBeIn...); got != tt.want {
				t.Errorf("allIn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func fixtures(t *testing.T) *Fixtures {
	mockCtrl := gomock.NewController(t)

	f := Fixtures{
		persist:  mocks.NewMockPersistenceService(mockCtrl),
		eventbus: mocks.NewMockEventBus(mockCtrl),
		ctx:      context.Background(),
	}

	f.service = NewService(f.persist, f.eventbus)

	return &f
}

func (f *Fixtures) mockWorkerStatuses(workerStatuses persistence.WorkerStatusCount) {
	f.persist.EXPECT().SummarizeWorkerStatuses(f.ctx).Return(workerStatuses, nil)
}

func (f *Fixtures) mockJobStatuses(jobStatuses persistence.JobStatusCount) {
	f.persist.EXPECT().SummarizeJobStatuses(f.ctx).Return(jobStatuses, nil)
}
