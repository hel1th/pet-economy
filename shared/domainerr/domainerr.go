package domainerr

import "errors"

var (
	ErrNotFound     = errors.New("resource not found")
	ErrForbidden    = errors.New("access forbidden")
	ErrUnauthorized = errors.New("authentication required")
	ErrConflict     = errors.New("state conflict")
)

type InvalidError struct {
	Field   string
	Message string
}

func NewInvalid(field, message string) *InvalidError {
	return &InvalidError{Field: field, Message: message}
}

func (e *InvalidError) Error() string {
	return e.Field + ": " + e.Message
}

type ConflictError struct {
	Message string
}

func NewConflict(message string) *ConflictError {
	return &ConflictError{Message: message}
}

func (e *ConflictError) Error() string {
	return e.Message
}

func (e *ConflictError) Is(target error) bool {
	return errors.Is(target, ErrConflict)
}
