// SPDX-License-Identifier: GPL-3.0-or-later
package persistence

import (
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrJobNotFound       = PersistenceError{Message: "job not found", Err: sql.ErrNoRows}
	ErrTaskNotFound      = PersistenceError{Message: "task not found", Err: sql.ErrNoRows}
	ErrWorkerNotFound    = PersistenceError{Message: "worker not found", Err: sql.ErrNoRows}
	ErrWorkerTagNotFound = PersistenceError{Message: "worker tag not found", Err: sql.ErrNoRows}

	ErrDeletingWithoutFK = errors.New("refusing to delete a job when foreign keys are not enabled on the database")

	// ErrContextCancelled wraps the SQLite error "interrupted (9)". That error is
	// (as far as Sybren could figure out) caused by the context being closed.
	// Unfortunately there is no wrapping of the context error, so it's not
	// possible to determine whether it was due to a 'deadline exceeded' error or
	// another cancellation cause (like upstream HTTP connection closing).
	ErrContextCancelled = errors.New("context cancelled")
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
	return wrapError(translateJobError(errorToWrap), message, msgArgs...)
}

func taskError(errorToWrap error, message string, msgArgs ...interface{}) error {
	return wrapError(translateTaskError(errorToWrap), message, msgArgs...)
}

func workerError(errorToWrap error, message string, msgArgs ...interface{}) error {
	return wrapError(translateWorkerError(errorToWrap), message, msgArgs...)
}

func workerTagError(errorToWrap error, message string, msgArgs ...interface{}) error {
	return wrapError(translateWorkerTagError(errorToWrap), message, msgArgs...)
}

func wrapError(errorToWrap error, message string, format ...interface{}) error {
	// Only format if there are arguments for formatting.
	var formattedMsg string
	if len(format) > 0 {
		formattedMsg = fmt.Sprintf(message, format...)
	} else {
		formattedMsg = message
	}

	// Translate the SQLite "interrupted" error into something the error-handling
	// code can check for.
	if errorToWrap.Error() == "interrupted (9)" {
		errorToWrap = ErrContextCancelled
	}

	return PersistenceError{
		Message: formattedMsg,
		Err:     errorToWrap,
	}
}

// translateJobError translates a Gorm error to a persistence layer error.
// This helps to keep Gorm as "implementation detail" of the persistence layer.
func translateJobError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrJobNotFound
	}
	return err
}

// translateTaskError translates a Gorm error to a persistence layer error.
// This helps to keep Gorm as "implementation detail" of the persistence layer.
func translateTaskError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTaskNotFound
	}
	return err
}

// translateWorkerError translates a Gorm error to a persistence layer error.
// This helps to keep Gorm as "implementation detail" of the persistence layer.
func translateWorkerError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrWorkerNotFound
	}
	return err
}

// translateWorkerTagError translates a Gorm error to a persistence layer error.
// This helps to keep Gorm as "implementation detail" of the persistence layer.
func translateWorkerTagError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrWorkerTagNotFound
	}
	return err
}
