package domain

import "errors"

// Sentinel errors for domain-level conditions.
var (
	// ErrNotFound is returned when a requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists is returned when attempting to create a duplicate entity.
	ErrAlreadyExists = errors.New("already exists")
	// ErrInvalidInput is returned when input validation fails.
	ErrInvalidInput = errors.New("invalid input")
)
