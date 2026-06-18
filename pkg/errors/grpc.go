package errors

import (
	stderrors "errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToStatus translates a domain error into a gRPC status. The signature is
// preserved for backward compatibility; the implementation now consults the
// Kind registry first and falls back to legacy sentinel switching.
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

// FromStatus converts a gRPC status back into a domain error. It is the
// adapter-side counterpart to ToStatus and lets callers leverage the same
// registry in both directions.
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

// Code returns the gRPC code for an error. It is a convenience adapter over
// ToStatus for callers that only need the code.
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

// IsKind reports whether err or any error it wraps carries the given Kind.
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
