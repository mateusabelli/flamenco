// SPDX-License-Identifier: GPL-3.0-or-later
package persistence

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotFoundErrors(t *testing.T) {
	assert.ErrorIs(t, ErrJobNotFound, sql.ErrNoRows)
	assert.ErrorIs(t, ErrTaskNotFound, sql.ErrNoRows)

	assert.Contains(t, ErrJobNotFound.Error(), "job")
	assert.Contains(t, ErrTaskNotFound.Error(), "task")
}

func TestTranslateJobError(t *testing.T) {
	assert.Nil(t, translateJobError(nil))
	assert.Equal(t, ErrJobNotFound, translateJobError(sql.ErrNoRows))

	otherError := errors.New("this error is not special for this function")
	assert.Equal(t, otherError, translateJobError(otherError))
}

func TestTranslateTaskError(t *testing.T) {
	assert.Nil(t, translateTaskError(nil))
	assert.Equal(t, ErrTaskNotFound, translateTaskError(sql.ErrNoRows))

	otherError := errors.New("this error is not special for this function")
	assert.Equal(t, otherError, translateTaskError(otherError))
}
