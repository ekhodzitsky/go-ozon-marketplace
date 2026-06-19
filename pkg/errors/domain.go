package errors

import (
	stderrors "errors"
	"fmt"
)

// Sentinel-ошибки — доменный словарь. Оставлены для совместимости и всё ещё
// используются в старом коде как основные точки сравнения.
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

// AppError — доменная ошибка. Поля Code, Message и Err оставлены для совместимости;
// Kind и Detail отделяют транспортно-нейтральную классификацию от текста для человека.
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

// New — старый конструктор. Для совместимости принимает строковый code,
// Kind подбирается из registry, если возможно.
func New(code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Kind:    ParseKind(code),
	}
}
