// SPDX-License-Identifier: GPL-3.0-or-later
package eventbus

import "fmt"

const (
	// Topics on which events are published.
	TopicJobUpdate         EventTopic = "/jobs"          // sends api.SocketIOJobUpdate
	TopicLastRenderedImage EventTopic = "/last-rendered" // sends api.SocketIOLastRenderedUpdate
	TopicTaskUpdate        EventTopic = "/task"          // sends api.SocketIOTaskUpdate
	TopicWorkerUpdate      EventTopic = "/workers"       // sends api.SocketIOWorkerUpdate
	TopicWorkerTagUpdate   EventTopic = "/workertags"    // sends api.SocketIOWorkerTagUpdate
	TopicSubscription      EventTopic = "/subscription"  // clients send api.SocketIOSubscription

	// Parameterised topics.
	TopicJobSpecific     EventTopic = "/jobs/%s"               // %s = job UUID
	TopicJobLastRendered EventTopic = "/jobs/%s/last-rendered" // %s = job UUID
	TopicTaskLog         EventTopic = "/tasklog/%s"            // %s = task UUID
)

// topicForJob will return the event topic for the given job. Clients subscribed
// to this topic receive info scoped to this job, so for example updates to all
// tasks of this job.
func topicForJob(jobUUID string) EventTopic {
	return EventTopic(fmt.Sprintf(string(TopicJobSpecific), jobUUID))
}
func topicForJobLastRendered(jobUUID string) EventTopic {
	return EventTopic(fmt.Sprintf(string(TopicJobLastRendered), jobUUID))
}

// topicForTaskLog will return the event topic for receiving task logs of
// the the given task.
//
// Note that general task updates are sent to their job's topic, and not to this
// one.
func topicForTaskLog(taskUUID string) EventTopic {
	return EventTopic(fmt.Sprintf(string(TopicTaskLog), taskUUID))
}
