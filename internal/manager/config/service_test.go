package config

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Replace(t *testing.T) {
	s := NewService()

	// Create a temp file to associate this config service with, so that we can
	// safely write to the file in this test, without being afraid of overwriting
	// an actual config file.
	tmpFile, err := os.CreateTemp("", "flamenco-manager-test-*.yaml")
	require.NoError(t, err)
	s.filename = tmpFile.Name()
	require.NoError(t, tmpFile.Close())

	// Check the precondition, that the default config has a "blender" variable,
	// and that the VariablesLookup table is set up.
	require.Equal(t, "blender", s.config.VariablesLookup[VariableAudienceWorkers][VariablePlatformLinux]["blender"])

	// Copy the config by roundtripping via JSON, as that's how API interaction would work.
	var newConfig Conf
	configBytes, err := json.Marshal(s.config)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(configBytes, &newConfig))
	require.Nil(t, newConfig.VariablesLookup, "VariablesLookup should not be serialized to JSON")

	// Update the new config's Blender variable.
	require.Equal(t, VariablePlatformLinux, newConfig.Variables["blender"].Values[0].Platform)
	require.Equal(t, VariableAudienceAll, newConfig.Variables["blender"].Values[0].Audience)
	newConfig.Variables["blender"].Values[0].Value = "linux-blender"
	require.NoError(t, s.Replace(newConfig))

	// Check that the new config was loaded correctly.
	require.Equal(t, VariablePlatformLinux, newConfig.Variables["blender"].Values[0].Platform)
	require.Equal(t, VariableAudienceAll, newConfig.Variables["blender"].Values[0].Audience)
	assert.Equal(t, "linux-blender", s.config.Variables["blender"].Values[0].Value,
		"The variable value should have been updated")
	assert.Equal(t, "linux-blender", s.config.VariablesLookup[VariableAudienceWorkers][VariablePlatformLinux]["blender"],
		"config.VariablesLookup should have been updated")

	// If the test failed, keep the file around for inspection.
	if !t.Failed() {
		_ = os.Remove(s.filename)
	}
}
