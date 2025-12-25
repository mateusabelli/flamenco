package worker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"projects.blender.org/studio/flamenco/internal/worker/mocks"
	"projects.blender.org/studio/flamenco/pkg/api"
)

type taskExecutorWithMocks struct {
	te        *TaskExecutor
	cmdRunner *mocks.MockCommandRunner
	listener  *mocks.MockTaskExecutionListener
	ctx       context.Context
	cleanup   func()
}

func newTaskExecutorWithMocks(t *testing.T) *taskExecutorWithMocks {
	mockCtrl := gomock.NewController(t)

	ctx, cancel := context.WithCancel(context.Background())
	mocks := taskExecutorWithMocks{
		cmdRunner: mocks.NewMockCommandRunner(mockCtrl),
		listener:  mocks.NewMockTaskExecutionListener(mockCtrl),
		ctx:       ctx,
		cleanup:   cancel,
	}
	te := NewTaskExecutor(mocks.cmdRunner, mocks.listener)

	mocks.te = te
	return &mocks
}

func TestTaskExecutor_Run(t *testing.T) {
	mte := newTaskExecutorWithMocks(t)
	defer mte.cleanup()

	cmd1 := api.Command{
		Name:       "cmd-without-steps",
		Parameters: map[string]interface{}{"message": "this is a test"},
	}
	cmd2 := api.Command{
		Name:           "cmd-with-steps",
		Parameters:     map[string]interface{}{"message": "this is a test"},
		TotalStepCount: 3,
	}

	task := api.AssignedTask{
		Commands:       []api.Command{cmd1, cmd2},
		StepsCompleted: 0,
		StepsTotal:     4,

		Job:         "4fe3f7a7-a712-445b-9041-b0d7ef0b1912",
		JobPriority: 1,
		JobType:     "exe-test",
		Name:        "Execution Test",
		Priority:    50,
		Status:      api.TaskStatusActive,
		TaskType:    "typie-task",
		Uuid:        "5f6725a6-be1a-45bd-8cef-783e1480e3f5",
	}

	// Set up expectations.
	mte.listener.EXPECT().TaskStarted(mte.ctx, task.Uuid)
	mte.cmdRunner.EXPECT().Run(mte.ctx, task.Uuid, cmd1)
	mte.listener.EXPECT().TaskStep(mte.ctx, task.Uuid)
	mte.cmdRunner.EXPECT().Run(mte.ctx, task.Uuid, cmd2)
	// cmd2 should not cause a call to TaskStep(), because it has a TotalStepCount > 0 and
	// thus should do its own counting. That's done in the runner, which is mocked here,
	// hence no expected call.
	mte.listener.EXPECT().TaskCompleted(mte.ctx, task.Uuid)

	require.NoError(t, mte.te.Run(mte.ctx, task))
}
