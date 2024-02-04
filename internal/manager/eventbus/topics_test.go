package eventbus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParamterisedTopics(t *testing.T) {
	uuid := "646f85f3-7166-40bd-9d43-d2b5baaeb42a"
	assert.Equal(t, EventTopic("/jobs/646f85f3-7166-40bd-9d43-d2b5baaeb42a"), topicForJob(uuid))
	assert.Equal(t, EventTopic("/jobs/646f85f3-7166-40bd-9d43-d2b5baaeb42a/last-rendered"), topicForJobLastRendered(uuid))
	assert.Equal(t, EventTopic("/tasklog/646f85f3-7166-40bd-9d43-d2b5baaeb42a"), topicForTaskLog(uuid))
}
