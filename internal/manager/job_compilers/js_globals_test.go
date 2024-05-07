package job_compilers

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellSplitHappy(t *testing.T) {
	expect := []string{"--python-expr", "print(1 + 1)"}
	actual := jsShellSplit(nil, "--python-expr 'print(1 + 1)'")
	assert.Equal(t, expect, actual)
}

func TestShellSplitFailure(t *testing.T) {
	vm := goja.New()

	testFunc := func() {
		jsShellSplit(vm, "--python-expr invalid_quoting(1 + 1)'")
	}
	// Testing that a goja.Value is used for the panic is a bit tricky, so just
	// test that the function panics.
	assert.Panics(t, testFunc)
}

func TestFrameChunkerHappyBlenderStyle(t *testing.T) {
	chunks, err := jsFrameChunker("1..10,20..25,40,3..8", 4)
	require.NoError(t, err)
	assert.Equal(t, []string{"1-4", "5-8", "9,10,20,21", "22-25", "40"}, chunks)
}

func TestFrameChunkerHappySmallInput(t *testing.T) {
	// No frames, should be an error
	_, err := jsFrameChunker("   ", 4)
	assert.ErrorIs(t, err, ErrInvalidRange{Message: "empty range"})

	// Just one frame.
	chunks, err := jsFrameChunker("47", 4)
	require.NoError(t, err)
	assert.Equal(t, []string{"47"}, chunks)

	// Just one range of exactly one chunk.
	chunks, err = jsFrameChunker("1-3", 3)
	require.NoError(t, err)
	assert.Equal(t, []string{"1-3"}, chunks)
}

func TestFrameChunkerHappyRegularStyle(t *testing.T) {
	chunks, err := jsFrameChunker("1-10,20-25,40", 4)
	require.NoError(t, err)
	assert.Equal(t, []string{"1-4", "5-8", "9,10,20,21", "22-25", "40"}, chunks)
}

func TestFrameChunkerHappyExtraWhitespace(t *testing.T) {
	chunks, err := jsFrameChunker(" 1  .. 10,\t20..25\n,40   ", 4)
	require.NoError(t, err)
	assert.Equal(t, []string{"1-4", "5-8", "9,10,20,21", "22-25", "40"}, chunks)
}

func TestFrameChunkerUnhappy(t *testing.T) {
	_, err := jsFrameChunker(" 1 10", 4)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "1 10")
}

func TestFrameRangeExplode(t *testing.T) {
	frames, err := frameRangeExplode("1..10,20..25,40")
	require.NoError(t, err)
	assert.Equal(t, []int{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
		20, 21, 22, 23, 24, 25, 40,
	}, frames)
}
