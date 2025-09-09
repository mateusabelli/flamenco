package worker

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"

	"projects.blender.org/studio/flamenco/pkg/api"
)

// SPDX-License-Identifier: GPL-3.0-or-later

func TestCmdBlenderSimpleCliArgs(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ce, mocks := testCommandExecutor(t, mockCtrl)

	taskID := "1d54c6fe-1242-4c8f-bd63-5a09e358d7b6"
	cmd := api.Command{
		Name: "blender",
		Parameters: map[string]interface{}{
			"exe":        "/path/to/blender",
			"argsBefore": []string{"--background"},
			"blendfile":  "file.blend",
			"args":       []string{"--render-output", "/frames"},
		},
	}

	cliArgs := []string{"--background", "file.blend", "--render-output", "/frames"}
	mocks.cli.EXPECT().CommandContext(gomock.Any(), "/path/to/blender", cliArgs).Return(nil)

	err := ce.cmdBlenderRender(context.Background(), zerolog.Nop(), taskID, cmd)
	assert.Equal(t, ErrNoExecCmd, err, "nil *exec.Cmd should result in ErrNoExecCmd")
}

func TestCmdBlenderCliArgsInExeParameter(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ce, mocks := testCommandExecutor(t, mockCtrl)

	// Just use backslashes even when we're testing on Linux/macOS. Backslashes
	// are always causing issues, so it's better to test with those on other
	// platforms as well. Even when the resulting command cannot be run, every
	// platform should be able to at least make the backslashes survive any
	// processing.
	exe := `D:\Blender_3.2_stable\blender.exe`

	taskID := "1d54c6fe-1242-4c8f-bd63-5a09e358d7b6"
	cmd := api.Command{
		Name: "blender",
		Parameters: map[string]interface{}{
			"exe": exe,
			// This intentionally starts with a space (should be ignored) and has
			// quoted text within quoted text:
			"exeArgs":    ` --factory-startup --python-expr "import bpy; print(\"hello world\")"`,
			"argsBefore": []string{"-no-audio"},
			"blendfile":  "file with spaces.blend",
			"args":       []string{"--debug"},
		},
	}

	mocks.cli.EXPECT().CommandContext(gomock.Any(),
		exe,                                // from 'exe'
		"--factory-startup",                // from 'exeArgs'
		"--python-expr",                    // from 'exeArgs'
		`import bpy; print("hello world")`, // from 'exeArgs'
		"-no-audio",                        // from 'argsBefore'
		"file with spaces.blend",           // from 'blendfile'
		"--debug",                          // from 'args'
	).Return(nil)

	err := ce.cmdBlenderRender(context.Background(), zerolog.Nop(), taskID, cmd)
	assert.Equal(t, ErrNoExecCmd, err, "nil *exec.Cmd should result in ErrNoExecCmd")
}

func TestCmdBlenderNoBlendfile(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ce, mocks := testCommandExecutor(t, mockCtrl)

	taskID := "1d54c6fe-1242-4c8f-bd63-5a09e358d7b6"
	cmd := api.Command{
		Name: "blender",
		Parameters: map[string]interface{}{
			"exe":        "/path/to/blender",
			"argsBefore": []string{"--background"},
			"blendfile":  "", // Empty blendfile should be skipped.
			"args":       []string{"--render-output", "/frames"},
		},
	}

	cliArgs := []string{"--background", "--render-output", "/frames"}
	mocks.cli.EXPECT().CommandContext(gomock.Any(), "/path/to/blender", cliArgs).Return(nil)

	err := ce.cmdBlenderRender(context.Background(), zerolog.Nop(), taskID, cmd)
	assert.Equal(t, ErrNoExecCmd, err, "nil *exec.Cmd should result in ErrNoExecCmd")
}

// TestProcessLineBlender45 tests Blender 4.5 (and older) logging output.
func TestProcessLineBlender45(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ce, mocks := testCommandExecutor(t, mockCtrl)
	taskID := "c194ea21-1fda-46f6-bc9a-34bd302cfb19"

	// This shouldn't call anything on the mocks.
	ce.processLineBlender(ctx, log.Logger, taskID, "starting Blender")

	// This should be recognised as produced output.
	mocks.listener.EXPECT().OutputProduced(ctx, taskID, "/path/to/file.exr")
	ce.processLineBlender(ctx, log.Logger, taskID, "Saved: '/path/to/file.exr'")

	// This should be recognised as being done with a frame.
	mocks.listener.EXPECT().TaskStep(ctx, taskID)
	ce.processLineBlender(ctx, log.Logger, taskID, "Time: 00:01.06")

	// This should be recognised as skipping a frame.
	mocks.listener.EXPECT().TaskStep(ctx, taskID)
	ce.processLineBlender(ctx, log.Logger, taskID, `Skipping existing frame "/path/to/running-default-cube0001.jpg"`)
}

// TestProcessLineBlender50 tests Blender 5.0 logging output.
func TestProcessLineBlender50(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ce, mocks := testCommandExecutor(t, mockCtrl)
	taskID := "c194ea21-1fda-46f6-bc9a-34bd302cfb19"

	// This shouldn't call anything on the mocks.
	ce.processLineBlender(ctx, log.Logger, taskID, "starting Blender")

	// This should be recognised as produced output.
	mocks.listener.EXPECT().OutputProduced(ctx, taskID, "/path/to/file.exr")
	ce.processLineBlender(ctx, log.Logger, taskID,
		"00:00.327  render           | Saved: '/path/to/file.exr'")

	// This should be recognised as being done with a frame.
	mocks.listener.EXPECT().TaskStep(ctx, taskID)
	ce.processLineBlender(ctx, log.Logger, taskID,
		"00:00.419  render           | Time: 00:01.06 (Saving: 00:00.02)")

	// This should be recognised as skipping a frame.
	mocks.listener.EXPECT().TaskStep(ctx, taskID)
	ce.processLineBlender(ctx, log.Logger, taskID,
		`00:00.419  render           | Skipping existing frame "/path/to/running-default-cube0001.jpg"`)
}
