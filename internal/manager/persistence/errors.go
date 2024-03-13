// SPDX-License-Identifier: GPL-3.0-or-later
package persistence

import (
	"database/sql"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var (
	// TODO: let these errors wrap database/sql.ErrNoRows.
	ErrJobNotFound       = PersistenceError{Message: "job not found", Err: gorm.ErrRecordNotFound}
	ErrTaskNotFound      = PersistenceError{Message: "task not found", Err: gorm.ErrRecordNotFound}
	ErrWorkerNotFound    = PersistenceError{Message: "worker not found", Err: gorm.ErrRecordNotFound}
	ErrWorkerTagNotFound = PersistenceError{Message: "worker tag not found", Err: gorm.ErrRecordNotFound}

	ErrDeletingWithoutFK = errors.New("refusing to delete a job when foreign keys are not enabled on the database")
)

type PersistenceError struct {
	Message string // The error message.
	Err     error  // Any wrapped error.
}

func (e PersistenceError) Error() string {
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e PersistenceError) Is(err error) bool {
	return err == e.Err
}

func jobError(errorToWrap error, message string, msgArgs ...interface{}) error {
	return wrapError(translateGormJobError(errorToWrap), message, msgArgs...)
}

func taskError(errorToWrap error, message string, msgArgs ...interface{}) error {
	return wrapError(translateGormTaskError(errorToWrap), message, msgArgs...)
}

func workerError(errorToWrap error, message string, msgArgs ...interface{}) error {
	return wrapError(translateGormWorkerError(errorToWrap), message, msgArgs...)
}

func workerTagError(errorToWrap error, message string, msgArgs ...interface{}) error {
	return wrapError(translateGormWorkerTagError(errorToWrap), message, msgArgs...)
}

func wrapError(errorToWrap error, message string, format ...interface{}) error {
	// Only format if there are arguments for formatting.
	var formattedMsg string
	if len(format) > 0 {
		formattedMsg = fmt.Sprintf(message, format...)
	} else {
		formattedMsg = message
	}

	return PersistenceError{
		Message: formattedMsg,
		Err:     errorToWrap,
	}
}

// translateGormJobError translates a Gorm error to a persistence layer error.
// This helps to keep Gorm as "implementation detail" of the persistence layer.
func translateGormJobError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTaskNotFound
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrJobNotFound
	}
	return err
}

// translateGormTaskError translates a Gorm error to a persistence layer error.
// This helps to keep Gorm as "implementation detail" of the persistence layer.
func translateGormTaskError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTaskNotFound
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrTaskNotFound
	}
	return err
}

// translateGormWorkerError translates a Gorm error to a persistence layer error.
// This helps to keep Gorm as "implementation detail" of the persistence layer.
func translateGormWorkerError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrWorkerNotFound
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrWorkerNotFound
	}
	return err
}

// translateGormWorkerTagError translates a Gorm error to a persistence layer error.
// This helps to keep Gorm as "implementation detail" of the persistence layer.
func translateGormWorkerTagError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrWorkerTagNotFound
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrWorkerTagNotFound
	}
	return err
}
