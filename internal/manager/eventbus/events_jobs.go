package eventbus

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

// NewJobUpdate returns a partial SocketIOJobUpdate struct for the given job.
// It only fills in the fields that represent the current state of the job. For
// example, it omits `PreviousStatus`. The ommitted fields can be filled in by
// the caller.
func NewJobUpdate(job *persistence.Job) api.SocketIOJobUpdate {
	jobUpdate := api.SocketIOJobUpdate{
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
func NewTaskUpdate(task *persistence.Task) api.SocketIOTaskUpdate {
	taskUpdate := api.SocketIOTaskUpdate{
		Id:       task.UUID,
		JobId:    task.Job.UUID,
		Name:     task.Name,
		Updated:  task.UpdatedAt,
		Status:   task.Status,
		Activity: task.Activity,
	}
	return taskUpdate
}

// NewLastRenderedUpdate returns a partial SocketIOLastRenderedUpdate struct.
// The `Thumbnail` field still needs to be filled in, but that requires
// information from the `api_impl.Flamenco` service.
func NewLastRenderedUpdate(jobUUID string) api.SocketIOLastRenderedUpdate {
	return api.SocketIOLastRenderedUpdate{
		JobId: jobUUID,
	}
}

// NewTaskLogUpdate returns a SocketIOTaskLogUpdate for the given task.
func NewTaskLogUpdate(taskUUID string, logchunk string) api.SocketIOTaskLogUpdate {
	return api.SocketIOTaskLogUpdate{
		TaskId: taskUUID,
		Log:    logchunk,
	}
}

// BroadcastNewJob sends a "new job" notification to clients.
// This function should be called when the job has been completely created, so
// including its tasks.
func (b *Broker) BroadcastNewJob(jobUpdate api.SocketIOJobUpdate) {
	if jobUpdate.PreviousStatus != nil {
		log.Warn().Interface("jobUpdate", jobUpdate).Msg("socketIO: new jobs should not have a previous state")
		jobUpdate.PreviousStatus = nil
	}

	log.Debug().Interface("jobUpdate", jobUpdate).Msg("socketIO: broadcasting new job")
	b.broadcast(TopicJobUpdate, jobUpdate)
}

func (b *Broker) BroadcastJobUpdate(jobUpdate api.SocketIOJobUpdate) {
	log.Debug().Interface("jobUpdate", jobUpdate).Msg("socketIO: broadcasting job update")
	b.broadcast(TopicJobUpdate, jobUpdate)
}

func (b *Broker) BroadcastLastRenderedImage(update api.SocketIOLastRenderedUpdate) {
	log.Debug().Interface("lastRenderedUpdate", update).Msg("socketIO: broadcasting last-rendered image update")
	topic := topicForJobLastRendered(update.JobId)
	b.broadcast(topic, update)

	// TODO: throttle these via a last-in-one-out queue (see `pkg/last_in_one_out_queue`).
	b.broadcast(TopicLastRenderedImage, update)
}

func (b *Broker) BroadcastTaskUpdate(taskUpdate api.SocketIOTaskUpdate) {
	log.Debug().Interface("taskUpdate", taskUpdate).Msg("socketIO: broadcasting task update")
	topic := topicForJob(taskUpdate.JobId)
	b.broadcast(topic, taskUpdate)
}

func (b *Broker) BroadcastTaskLogUpdate(taskLogUpdate api.SocketIOTaskLogUpdate) {
	// Don't log the contents here; logs can get big.
	topic := topicForTaskLog(taskLogUpdate.TaskId)
	log.Debug().
		Str("task", taskLogUpdate.TaskId).
		Str("topic", string(topic)).
		Msg("socketIO: broadcasting task log")
	b.broadcast(topic, taskLogUpdate)
}
