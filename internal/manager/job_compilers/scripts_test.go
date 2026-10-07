package job_compilers

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadScriptsFrom_skip_nonjs(t *testing.T) {
	thisDirFS := os.DirFS(".")
	compilers, err := loadScriptsFrom(thisDirFS)
	require.NoError(t, err, "input without JS files should not cause errors")
	assert.Empty(t, compilers)
}

func TestLoadScriptsFrom_on_disk_js(t *testing.T) {
	scriptsFS := os.DirFS("scripts-for-unittest")
	compilers, err := loadScriptsFrom(scriptsFS)

	require.NoError(t, err)
	expectKeys := map[string]bool{
		"echo-and-sleep":        true,
		"simple-blender-render": true,
		// Should NOT contain an entry for 'empty.js'.
	}
	assert.Equal(t, expectKeys, keys(compilers))
}

func TestLoadScriptsFrom_on_disk_symlink(t *testing.T) {
	// Symlink a script file to a supported filename.
	targetName := "scripts-for-unittest/symlink_me.js"
	_ = os.Remove(targetName) // Remove in case a previous run didn't clean up.
	require.NoError(t, os.Symlink("symlink_me.js.txt", targetName))
	t.Cleanup(func() {
		require.NoError(t, os.Remove(targetName))
	})

	scriptsFS := os.DirFS("scripts-for-unittest")
	compilers, err := loadScriptsFrom(scriptsFS)

	require.NoError(t, err)
	expectKeys := map[string]bool{
		"echo-and-sleep":        true,
		"simple-blender-render": true,
		"symlink-me":            true,
	}
	assert.Equal(t, expectKeys, keys(compilers))
}

func TestLoadScriptsFrom_embedded(t *testing.T) {
	initEmbeddedFS()
	compilers, err := loadScriptsFrom(embeddedScriptsFS)

	require.NoError(t, err)
	expectKeys := map[string]bool{
		"echo-sleep-test":       true,
		"simple-blender-render": true,
	}
	assert.Equal(t, expectKeys, keys(compilers))
}

func BenchmarkLoadScripts_fromEmbedded(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	initEmbeddedFS()

	for b.Loop() {
		compilers, err := loadScriptsFrom(embeddedScriptsFS)
		require.NoError(b, err)
		assert.NotEmpty(b, compilers)
	}
}

func BenchmarkLoadScripts_fromDisk(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	onDiskFS := os.DirFS("scripts-for-unittest")
	for b.Loop() {
		compilers, err := loadScriptsFrom(onDiskFS)
		require.NoError(b, err)
		assert.NotEmpty(b, compilers)
	}
}

func BenchmarkLoadScripts(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	for b.Loop() {
		loadScripts()
	}
}

// keys returns the set of keys of the mapping.
func keys[K comparable, V any](mapping map[K]V) map[K]bool {
	keys := map[K]bool{}
	for k := range mapping {
		keys[k] = true
	}
	return keys
}
