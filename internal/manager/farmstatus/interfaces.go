package farmstatus

import (
	"context"

	"projects.blender.org/studio/flamenco/internal/manager/eventbus"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// Generate mock implementations of these interfaces.
//go:generate go run github.com/golang/mock/mockgen -destination mocks/interfaces_mock.gen.go -package mocks projects.blender.org/studio/flamenco/internal/manager/farmstatus PersistenceService,EventBus

type PersistenceService interface {
	SummarizeJobStatuses(ctx context.Context) (persistence.JobStatusCount, error)
	SummarizeWorkerStatuses(ctx context.Context) (persistence.WorkerStatusCount, error)
}

var _ PersistenceService = (*persistence.DB)(nil)

type EventBus interface {
	BroadcastFarmStatusEvent(event api.EventFarmStatus)
}

var _ EventBus = (*eventbus.Broker)(nil)
