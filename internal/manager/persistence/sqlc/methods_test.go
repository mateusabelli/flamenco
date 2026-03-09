package sqlc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkerTaskTypes(t *testing.T) {
	{ // Multiple task types.
		w := Worker{SupportedTaskTypes: "ffmpeg,blender"}
		assert.Equal(t, []string{"ffmpeg", "blender"}, w.TaskTypes())
	}
	{ // Single task type.
		w := Worker{SupportedTaskTypes: "ffmpeg"}
		assert.Equal(t, []string{"ffmpeg"}, w.TaskTypes())
	}
	{ // No task types.
		w := Worker{SupportedTaskTypes: ""}
		assert.Equal(t, []string{}, w.TaskTypes())
	}
}
