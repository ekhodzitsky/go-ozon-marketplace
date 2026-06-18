package errors

import "fmt"

// E creates a domain error from a Kind. It is the preferred constructor for
// new code because it makes the transport-neutral classification explicit.
func E(kind Kind, message string) *AppError {
	return &AppError{
		Code:    string(kind),
		Kind:    kind,
		Message: message,
	}
}

// Ef is the formatted variant of E.
func Ef(kind Kind, format string, args ...interface{}) *AppError {
	return E(kind, fmt.Sprintf(format, args...))
}

// Wrap attaches an underlying error to a domain error. The code argument is
// kept as a string for backward compatibility and mapped to a Kind when known.
func Wrap(err error, code, message string) *AppError {
	return &AppError{
		Code:    code,
		Kind:    ParseKind(code),
		Message: message,
		Err:     err,
	}
}

// Wrapf is the formatted variant of Wrap.
func Wrapf(err error, code, format string, args ...interface{}) *AppError {
	return Wrap(err, code, fmt.Sprintf(format, args...))
}

// WithDetail annotates an error with machine-readable detail. When applied to
// an AppError it returns a shallow copy so the original value is untouched.
func WithDetail(err error, detail string) error {
	if err == nil {
		return nil
	}
	if ae, ok := err.(*AppError); ok {
		cp := *ae
		cp.Detail = detail
		return &cp
	}
	return &AppError{
		Kind:    KindInternal,
		Code:    string(KindInternal),
		Message: err.Error(),
		Detail:  detail,
		Err:     err,
	}
}
