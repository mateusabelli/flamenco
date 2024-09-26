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

func TestTranslateGormJobError(t *testing.T) {
	assert.Nil(t, translateGormJobError(nil))
	assert.Equal(t, ErrJobNotFound, translateGormJobError(sql.ErrNoRows))

	otherError := errors.New("this error is not special for this function")
	assert.Equal(t, otherError, translateGormJobError(otherError))
}

func TestTranslateGormTaskError(t *testing.T) {
	assert.Nil(t, translateGormTaskError(nil))
	assert.Equal(t, ErrTaskNotFound, translateGormTaskError(sql.ErrNoRows))

	otherError := errors.New("this error is not special for this function")
	assert.Equal(t, otherError, translateGormTaskError(otherError))
}
