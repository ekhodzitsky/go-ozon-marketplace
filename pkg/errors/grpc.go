package errors

import (
	stderrors "errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToStatus превращает доменную ошибку в gRPC-статус. Сигнатура сохранена
// для совместимости; реализация сначала смотрит в registry Kind, а потом
// откатывается на старое переключение по sentinel-ошибкам.
func ToStatus(err error) error {
	if err == nil {
		return nil
	}

	if ae, ok := err.(*AppError); ok {
		entry, ok := EntryFor(ae.Kind)
		if !ok && ae.Code != "" {
			entry, ok = EntryFor(ParseKind(ae.Code))
		}
		if ok {
			return status.Error(entry.GRPCCode, ae.Error())
		}
	}

	switch {
	case stderrors.Is(err, ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case stderrors.Is(err, ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case stderrors.Is(err, ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case stderrors.Is(err, ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case stderrors.Is(err, ErrPermissionDenied):
		return status.Error(codes.PermissionDenied, err.Error())
	case stderrors.Is(err, ErrConflict), stderrors.Is(err, ErrInsufficientStock), stderrors.Is(err, ErrFailedPrecondition):
		return status.Error(codes.FailedPrecondition, err.Error())
	case stderrors.Is(err, ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case stderrors.Is(err, ErrUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	case stderrors.Is(err, ErrDeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

// FromStatus превращает gRPC-статус обратно в доменную ошибку.
// Это зеркало к ToStatus: тот же registry работает в обе стороны.
func FromStatus(err error) *AppError {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return &AppError{
			Kind:    KindInternal,
			Code:    string(KindInternal),
			Message: err.Error(),
		}
	}
	kind := kindFromGRPC(st.Code())
	return &AppError{
		Kind:    kind,
		Code:    string(kind),
		Message: st.Message(),
	}
}

// Code возвращает gRPC-код ошибки. Удобная обёртка над ToStatus, если нужен только код.
func Code(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	st, ok := status.FromError(ToStatus(err))
	if !ok {
		return codes.Internal
	}
	return st.Code()
}

// IsKind проверяет, что err или любая из обёрнутых в неё ошибок имеет заданный Kind.
func IsKind(err error, kind Kind) bool {
	for err != nil {
		if ae, ok := err.(*AppError); ok && ae.Kind == kind {
			return true
		}
		err = stderrors.Unwrap(err)
	}
	return false
}

func kindFromGRPC(c codes.Code) Kind {
	switch c {
	case codes.NotFound:
		return KindNotFound
	case codes.AlreadyExists:
		return KindAlreadyExists
	case codes.InvalidArgument:
		return KindInvalidArgument
	case codes.Unauthenticated:
		return KindUnauthenticated
	case codes.PermissionDenied:
		return KindPermissionDenied
	case codes.FailedPrecondition:
		return KindFailedPrecondition
	case codes.Unavailable:
		return KindUnavailable
	case codes.DeadlineExceeded:
		return KindDeadlineExceeded
	default:
		return KindInternal
	}
}
