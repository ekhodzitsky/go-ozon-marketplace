package errors

import (
	stderrors "errors"
	"fmt"
)

// Sentinel errors form the domain vocabulary. They are preserved for backward
// compatibility and remain the primary handle for legacy code paths.
var (
	ErrNotFound           = stderrors.New("not found")
	ErrAlreadyExists      = stderrors.New("already exists")
	ErrInvalidArgument    = stderrors.New("invalid argument")
	ErrInvalidCredentials = stderrors.New("invalid credentials")
	ErrPermissionDenied   = stderrors.New("permission denied")
	ErrConflict           = stderrors.New("conflict")
	ErrInsufficientStock  = stderrors.New("insufficient stock")
	ErrFailedPrecondition = stderrors.New("failed precondition")
	ErrUnauthenticated    = stderrors.New("unauthenticated")
	ErrUnavailable        = stderrors.New("unavailable")
	ErrDeadlineExceeded   = stderrors.New("deadline exceeded")
)

// AppError is the domain error carrier. Exported fields Code, Message, and Err
// are retained for backward compatibility; Kind and Detail deepen the module
// by separating transport-neutral classification from human-readable messaging.
type AppError struct {
	Code    string
	Message string
	Err     error
	Kind    Kind
	Detail  string
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// New is the legacy constructor. It stays backward-compatible by accepting a
// string code and deriving the Kind from the registry when possible.
func New(code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Kind:    ParseKind(code),
	}
}
