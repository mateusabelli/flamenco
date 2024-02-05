package eventbus

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// NewJobUpdate returns a partial EventJobUpdate struct for the given job.
// It only fills in the fields that represent the current state of the job. For
// example, it omits `PreviousStatus`. The ommitted fields can be filled in by
// the caller.
func NewJobUpdate(job *persistence.Job) api.EventJobUpdate {
	jobUpdate := api.EventJobUpdate{
		Id:       job.UUID,
		Name:     &job.Name,
		Updated:  job.UpdatedAt,
		Status:   job.Status,
		Type:     job.JobType,
		Priority: job.Priority,
	}

	if job.DeleteRequestedAt.Valid {
		jobUpdate.DeleteRequestedAt = &job.DeleteRequestedAt.Time
	}

	return jobUpdate
}

// NewTaskUpdate returns a partial TaskUpdate struct for the given task. It only
// fills in the fields that represent the current state of the task. For
// example, it omits `PreviousStatus`. The omitted fields can be filled in by
// the caller.
//
// Assumes task.Job is not nil.
func NewTaskUpdate(task *persistence.Task) api.EventTaskUpdate {
	taskUpdate := api.EventTaskUpdate{
		Id:       task.UUID,
		JobId:    task.Job.UUID,
		Name:     task.Name,
		Updated:  task.UpdatedAt,
		Status:   task.Status,
		Activity: task.Activity,
	}
	return taskUpdate
}

// NewLastRenderedUpdate returns a partial EventLastRenderedUpdate struct.
// The `Thumbnail` field still needs to be filled in, but that requires
// information from the `api_impl.Flamenco` service.
func NewLastRenderedUpdate(jobUUID string) api.EventLastRenderedUpdate {
	return api.EventLastRenderedUpdate{
		JobId: jobUUID,
	}
}

// NewTaskLogUpdate returns a EventTaskLogUpdate for the given task.
func NewTaskLogUpdate(taskUUID string, logchunk string) api.EventTaskLogUpdate {
	return api.EventTaskLogUpdate{
		TaskId: taskUUID,
		Log:    logchunk,
	}
}

// BroadcastNewJob sends a "new job" notification to clients.
// This function should be called when the job has been completely created, so
// including its tasks.
func (b *Broker) BroadcastNewJob(jobUpdate api.EventJobUpdate) {
	if jobUpdate.PreviousStatus != nil {
		log.Warn().Interface("jobUpdate", jobUpdate).Msg("eventbus: new jobs should not have a previous state")
		jobUpdate.PreviousStatus = nil
	}

	log.Debug().Interface("jobUpdate", jobUpdate).Msg("eventbus: broadcasting new job")
	b.broadcast(TopicJobUpdate, jobUpdate)
}

func (b *Broker) BroadcastJobUpdate(jobUpdate api.EventJobUpdate) {
	log.Debug().Interface("jobUpdate", jobUpdate).Msg("eventbus: broadcasting job update")
	b.broadcast(TopicJobUpdate, jobUpdate)
}

func (b *Broker) BroadcastLastRenderedImage(update api.EventLastRenderedUpdate) {
	log.Debug().Interface("lastRenderedUpdate", update).Msg("eventbus: broadcasting last-rendered image update")
	topic := topicForJobLastRendered(update.JobId)
	b.broadcast(topic, update)

	// TODO: throttle these via a last-in-one-out queue (see `pkg/last_in_one_out_queue`).
	b.broadcast(TopicLastRenderedImage, update)
}

func (b *Broker) BroadcastTaskUpdate(taskUpdate api.EventTaskUpdate) {
	log.Debug().Interface("taskUpdate", taskUpdate).Msg("eventbus: broadcasting task update")
	topic := topicForJob(taskUpdate.JobId)
	b.broadcast(topic, taskUpdate)
}

func (b *Broker) BroadcastTaskLogUpdate(taskLogUpdate api.EventTaskLogUpdate) {
	// Don't log the contents here; logs can get big.
	topic := topicForTaskLog(taskLogUpdate.TaskId)
	log.Debug().
		Str("task", taskLogUpdate.TaskId).
		Str("topic", string(topic)).
		Msg("eventbus: broadcasting task log")
	b.broadcast(topic, taskLogUpdate)
}
