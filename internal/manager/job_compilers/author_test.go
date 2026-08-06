package job_compilers

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorTaskAddDependencyTwice(t *testing.T) {
	author := Author{}

	A, _ := author.Task("A", "a")
	B, _ := author.Task("B", "b")

	err := A.AddDependency(B)
	require.NoError(t, err)

	A.AddDependency(B)

	// There should only be task B once in the dependency array.
	assert.Equal(t, A.Dependencies, []*AuthoredTask{B})
}

func TestAuthorTaskCircularDependencySimple(t *testing.T) {
	author := Author{}

	A, _ := author.Task("A", "a")
	B, _ := author.Task("B", "b")

	// A -> B
	err := A.AddDependency(B)
	require.NoError(t, err)

	// Circular dependency error B -> A.
	err = B.AddDependency(A)
	assert.ErrorIs(t, err, CircularTaskDependencyError{B.Name, A.Name})
}

func TestAuthorTaskCircularDependencyComplex(t *testing.T) {
	author := Author{}

	A, _ := author.Task("A", "a")
	B, _ := author.Task("B", "b")
	C, _ := author.Task("C", "c")

	// C -> B
	err := C.AddDependency(B)
	require.NoError(t, err)

	// C -> B -> A
	err = B.AddDependency(A)
	require.NoError(t, err)

	// C ────> B ────> A
	// ^               │
	// └───────────────┘
	err = A.AddDependency(C)
	assert.ErrorIs(t, err, CircularTaskDependencyError{A.Name, C.Name})
}
