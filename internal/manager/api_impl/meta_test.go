package api_impl

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"io/fs"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"projects.blender.org/studio/flamenco/internal/manager/config"
	"projects.blender.org/studio/flamenco/pkg/api"
)

func TestGetVariables(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	// Test Linux Worker.
	{
		resolvedVarsLinuxWorker := make(map[string]config.ResolvedVariable)
		resolvedVarsLinuxWorker["jobs"] = config.ResolvedVariable{
			IsTwoWay: true,
			Value:    "Linux value",
		}
		resolvedVarsLinuxWorker["blender"] = config.ResolvedVariable{
			IsTwoWay: false,
			Value:    "/usr/local/blender",
		}

		mf.config.EXPECT().
			ResolveVariables(config.VariableAudienceWorkers, config.VariablePlatformLinux).
			Return(resolvedVarsLinuxWorker)

		echoCtx := mf.prepareMockedRequest(nil)
		err := mf.flamenco.GetVariables(echoCtx, api.ManagerVariableAudienceWorkers, "linux")
		require.NoError(t, err)
		assertResponseJSON(t, echoCtx, http.StatusOK, api.ManagerVariables{
			AdditionalProperties: map[string]api.ManagerVariable{
				"blender": {Value: "/usr/local/blender", IsTwoway: false},
				"jobs":    {Value: "Linux value", IsTwoway: true},
			},
		})
	}

	// Test unknown platform User.
	{
		resolvedVarsUnknownPlatform := make(map[string]config.ResolvedVariable)
		mf.config.EXPECT().
			ResolveVariables(config.VariableAudienceUsers, config.VariablePlatform("troll")).
			Return(resolvedVarsUnknownPlatform)

		echoCtx := mf.prepareMockedRequest(nil)
		err := mf.flamenco.GetVariables(echoCtx, api.ManagerVariableAudienceUsers, "troll")
		require.NoError(t, err)
		assertResponseJSON(t, echoCtx, http.StatusOK, api.ManagerVariables{})
	}
}

func TestGetSharedStorage(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	conf := config.GetTestConfig(func(c *config.Conf) {
		// Test with a Manager on Windows.
		c.MockCurrentGOOSForTests("windows")

		// Set up a two-way variable to do the mapping.
		c.Variables["shared_storage_mapping"] = config.Variable{
			IsTwoWay: true,
			Values: []config.VariableValue{
				{Value: "/user/shared/storage", Platform: config.VariablePlatformLinux, Audience: config.VariableAudienceUsers},
				{Value: "/worker/shared/storage", Platform: config.VariablePlatformLinux, Audience: config.VariableAudienceWorkers},
				{Value: `S:\storage`, Platform: config.VariablePlatformWindows, Audience: config.VariableAudienceAll},
			},
		}
	})
	mf.config.EXPECT().Get().Return(&conf).AnyTimes()
	mf.config.EXPECT().EffectiveStoragePath().Return(`S:\storage\flamenco`).AnyTimes()

	{ // Test user client on Linux.
		// Defer to the actual ExpandVariables() implementation of the above config.
		mf.config.EXPECT().
			NewVariableExpander(config.VariableAudienceUsers, config.VariablePlatformLinux).
			DoAndReturn(conf.NewVariableExpander)
		mf.shaman.EXPECT().IsEnabled().Return(false)

		echoCtx := mf.prepareMockedRequest(nil)
		err := mf.flamenco.GetSharedStorage(echoCtx, api.ManagerVariableAudienceUsers, "linux")
		require.NoError(t, err)
		assertResponseJSON(t, echoCtx, http.StatusOK, api.SharedStorageLocation{
			Location: "/user/shared/storage/flamenco",
			Audience: api.ManagerVariableAudienceUsers,
			Platform: "linux",
		})
	}

	{ // Test worker client on Linux with Shaman enabled.
		// Defer to the actual ExpandVariables() implementation of the above config.
		mf.config.EXPECT().
			NewVariableExpander(config.VariableAudienceWorkers, config.VariablePlatformLinux).
			DoAndReturn(conf.NewVariableExpander)
		mf.shaman.EXPECT().IsEnabled().Return(true)

		echoCtx := mf.prepareMockedRequest(nil)
		err := mf.flamenco.GetSharedStorage(echoCtx, api.ManagerVariableAudienceWorkers, "linux")
		require.NoError(t, err)
		assertResponseJSON(t, echoCtx, http.StatusOK, api.SharedStorageLocation{
			Location:      "/worker/shared/storage/flamenco",
			Audience:      api.ManagerVariableAudienceWorkers,
			Platform:      "linux",
			ShamanEnabled: true,
		})
	}

	{ // Test user client on Windows.
		// Defer to the actual ExpandVariables() implementation of the above config.
		mf.config.EXPECT().
			NewVariableExpander(config.VariableAudienceUsers, config.VariablePlatformWindows).
			DoAndReturn(conf.NewVariableExpander)
		mf.shaman.EXPECT().IsEnabled().Return(false)

		echoCtx := mf.prepareMockedRequest(nil)
		err := mf.flamenco.GetSharedStorage(echoCtx, api.ManagerVariableAudienceUsers, "windows")
		require.NoError(t, err)
		assertResponseJSON(t, echoCtx, http.StatusOK, api.SharedStorageLocation{
			Location: `S:\storage\flamenco`,
			Audience: api.ManagerVariableAudienceUsers,
			Platform: "windows",
		})
	}

}

// Test shared storage sitting on /mnt/flamenco, where that's mapped to F:\ for Windows.
func TestGetSharedStorageDriveLetterRoot(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mf := newMockedFlamenco(mockCtrl)

	conf := config.GetTestConfig(func(c *config.Conf) {
		// Test with a Manager on Linux.
		c.MockCurrentGOOSForTests("linux")

		// Set up a two-way variable to do the mapping.
		c.Variables["shared_storage_mapping"] = config.Variable{
			IsTwoWay: true,
			Values: []config.VariableValue{
				{Value: "/mnt/flamenco", Platform: config.VariablePlatformLinux, Audience: config.VariableAudienceAll},
				{Value: `F:\`, Platform: config.VariablePlatformWindows, Audience: config.VariableAudienceAll},
			},
		}
	})
	mf.config.EXPECT().Get().Return(&conf).AnyTimes()
	mf.config.EXPECT().EffectiveStoragePath().Return(`/mnt/flamenco`).AnyTimes()

	{ // Test user client on Linux.
		mf.config.EXPECT().
			NewVariableExpander(config.VariableAudienceUsers, config.VariablePlatformLinux).
			DoAndReturn(conf.NewVariableExpander)
		mf.shaman.EXPECT().IsEnabled().Return(false)

		echoCtx := mf.prepareMockedRequest(nil)
		err := mf.flamenco.GetSharedStorage(echoCtx, api.ManagerVariableAudienceUsers, "linux")
		require.NoError(t, err)
		assertResponseJSON(t, echoCtx, http.StatusOK, api.SharedStorageLocation{
			Location: "/mnt/flamenco",
			Audience: api.ManagerVariableAudienceUsers,
			Platform: "linux",
		})
	}

	{ // Test user client on Windows.
		mf.config.EXPECT().
			NewVariableExpander(config.VariableAudienceUsers, config.VariablePlatformWindows).
			DoAndReturn(conf.NewVariableExpander)
		mf.shaman.EXPECT().IsEnabled().Return(false)

		echoCtx := mf.prepareMockedRequest(nil)
		err := mf.flamenco.GetSharedStorage(echoCtx, api.ManagerVariableAudienceUsers, "windows")
		require.NoError(t, err)
		assertResponseJSON(t, echoCtx, http.StatusOK, api.SharedStorageLocation{
			Location: `F:\`,
			Audience: api.ManagerVariableAudienceUsers,
			Platform: "windows",
		})
	}

}

func TestCheckSharedStoragePath(t *testing.T) {
	mf, finish := metaTestFixtures(t)
	defer finish()

	doTest := func(path string) echo.Context {
		echoCtx := mf.prepareMockedJSONRequest(
			api.PathCheckInput{Path: path})
		err := mf.flamenco.CheckSharedStoragePath(echoCtx)
		require.NoError(t, err)
		return echoCtx
	}

	// Test empty path.
	echoCtx := doTest("")
	assertResponseJSON(t, echoCtx, http.StatusOK, api.PathCheckResult{
		Path:     "",
		IsUsable: false,
		Cause:    "An empty path is not suitable as shared storage",
	})

	// Test usable path (well, at least readable & writable; it may not be shared via Samba/NFS).
	echoCtx = doTest(mf.tempdir)
	assertResponseJSON(t, echoCtx, http.StatusOK, api.PathCheckResult{
		Path:     mf.tempdir,
		IsUsable: true,
		Cause:    "Directory checked successfully",
	})
	files, err := filepath.Glob(filepath.Join(mf.tempdir, "*"))
	require.NoError(t, err)
	assert.Empty(t, files, "After a query, there should not be any leftovers")

	// Test inaccessible path.
	// For some reason, this doesn't work on Windows, and creating a file in
	// that directory is still allowed. The Explorer's properties panel of the
	// directory also shows "Read Only (only applies to files)", so at least
	// that seems consistent.
	// FIXME: find another way to test with unwritable directories on Windows.
	if runtime.GOOS != "windows" {

		// Root can always create directories, so this test would fail with a
		// confusing message. Instead it's better to refuse running as root at all.
		currentUser, err := user.Current()
		require.NoError(t, err)
		require.NotEqual(t, "0", currentUser.Uid,
			"this test requires running as normal user, not %s (%s)", currentUser.Username, currentUser.Uid)
		require.NotEqual(t, "root", currentUser.Username,
			"this test requires running as normal user, not %s (%s)", currentUser.Username, currentUser.Uid)

		parentPath := filepath.Join(mf.tempdir, "deep")
		testPath := filepath.Join(parentPath, "nesting")
		require.NoError(t, os.Mkdir(parentPath, fs.ModePerm))
		require.NoError(t, os.Mkdir(testPath, fs.FileMode(0)))

		echoCtx := doTest(testPath)
		result := api.PathCheckResult{}
		getResponseJSON(t, echoCtx, http.StatusOK, &result)
		assert.Equal(t, testPath, result.Path)
		assert.False(t, result.IsUsable)
		assert.Contains(t, result.Cause, "Unable to create a file")
	}
}

func TestSaveSetupAssistantConfig(t *testing.T) {
	mf, finish := metaTestFixtures(t)
	defer finish()

	defaultBlenderArgsVar := config.Variable{
		Values: config.VariableValues{
			{Platform: config.VariablePlatformAll, Value: config.DefaultBlenderArguments},
		},
	}

	doTest := func(body api.SetupAssistantConfig) config.Conf {
		// Always start the test with a clean configuration.
		originalConfig := config.DefaultConfig(func(c *config.Conf) {
			c.SharedStoragePath = ""
		})
		var savedConfig config.Conf

		// Mock the loading & saving of the config.
		mf.config.EXPECT().Get().Return(&originalConfig)
		mf.config.EXPECT().Save().Do(func() error {
			savedConfig = originalConfig
			return nil
		})

		// Call the API.
		echoCtx := mf.prepareMockedJSONRequest(body)
		err := mf.flamenco.SaveSetupAssistantConfig(echoCtx)
		require.NoError(t, err)

		assertResponseNoContent(t, echoCtx)
		return savedConfig
	}

	// Test situation where file association with .blend files resulted in a blender executable.
	{
		savedConfig := doTest(api.SetupAssistantConfig{
			StorageLocation: mf.tempdir,
			BlenderExecutable: api.BlenderPathCheckResult{
				IsUsable: true,
				Input:    "",
				Path:     "/path/to/blender",
				Source:   api.BlenderPathSourceFileAssociation,
			},
		})
		assert.Equal(t, mf.tempdir, savedConfig.SharedStoragePath)
		expectBlenderVar := config.Variable{
			Values: config.VariableValues{
				{Platform: "linux", Value: "blender"},
				{Platform: "windows", Value: "blender"},
				{Platform: "darwin", Value: "blender"},
			},
		}
		assert.Equal(t, expectBlenderVar, savedConfig.Variables["blender"])
		assert.Equal(t, defaultBlenderArgsVar, savedConfig.Variables["blenderArgs"])
	}

	// Test situation where the given command could be found on $PATH.
	{
		savedConfig := doTest(api.SetupAssistantConfig{
			StorageLocation: mf.tempdir,
			BlenderExecutable: api.BlenderPathCheckResult{
				IsUsable: true,
				Input:    "kitty",
				Path:     "/path/to/kitty",
				Source:   api.BlenderPathSourcePathEnvvar,
			},
		})
		assert.Equal(t, mf.tempdir, savedConfig.SharedStoragePath)
		expectBlenderVar := config.Variable{
			Values: config.VariableValues{
				{Platform: "linux", Value: "kitty"},
				{Platform: "windows", Value: "kitty"},
				{Platform: "darwin", Value: "kitty"},
			},
		}
		assert.Equal(t, expectBlenderVar, savedConfig.Variables["blender"])
		assert.Equal(t, defaultBlenderArgsVar, savedConfig.Variables["blenderArgs"])
	}

	// Test a custom command given with the full path.
	{
		savedConfig := doTest(api.SetupAssistantConfig{
			StorageLocation: mf.tempdir,
			BlenderExecutable: api.BlenderPathCheckResult{
				IsUsable: true,
				Input:    "/bin/cat",
				Path:     "/bin/cat",
				Source:   api.BlenderPathSourceInputPath,
			},
		})
		assert.Equal(t, mf.tempdir, savedConfig.SharedStoragePath)
		expectBlenderVar := config.Variable{
			Values: config.VariableValues{
				{Platform: "linux", Value: "/bin/cat"},
				{Platform: "windows", Value: "/bin/cat"},
				{Platform: "darwin", Value: "/bin/cat"},
			},
		}
		assert.Equal(t, expectBlenderVar, savedConfig.Variables["blender"])
		assert.Equal(t, defaultBlenderArgsVar, savedConfig.Variables["blenderArgs"])
	}

	// Test situation where adding a blender executable was skipped.
	{
		savedConfig := doTest(api.SetupAssistantConfig{
			StorageLocation: mf.tempdir,
			BlenderExecutable: api.BlenderPathCheckResult{
				IsUsable: true,
				Source:   api.BlenderPathSourceDefault,
			},
		})
		assert.Equal(t, mf.tempdir, savedConfig.SharedStoragePath)
		expectBlenderVar := config.Variable{
			Values: config.VariableValues{
				{Platform: "linux", Value: "blender"},
				{Platform: "windows", Value: "blender"},
				{Platform: "darwin", Value: "blender"},
			},
		}
		assert.Equal(t, expectBlenderVar, savedConfig.Variables["blender"])
		assert.Equal(t, defaultBlenderArgsVar, savedConfig.Variables["blenderArgs"])
	}
}

func metaTestFixtures(t *testing.T) (mockedFlamenco, func()) {
	mockCtrl := gomock.NewController(t)
	mf := newMockedFlamenco(mockCtrl)

	tempdir, err := os.MkdirTemp("", "test-temp-dir")
	require.NoError(t, err)
	mf.tempdir = tempdir

	finish := func() {
		mockCtrl.Finish()
		os.RemoveAll(tempdir)
	}

	return mf, finish
}
