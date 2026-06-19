package errors

import "fmt"

// E создаёт доменную ошибку из Kind. Предпочтительный конструктор для нового кода,
// потому что явно указывает транспортно-нейтральную классификацию.
func E(kind Kind, message string) *AppError {
	return &AppError{
		Code:    string(kind),
		Kind:    kind,
		Message: message,
	}
}

// Ef — форматированный вариант E.
func Ef(kind Kind, format string, args ...interface{}) *AppError {
	return E(kind, fmt.Sprintf(format, args...))
}

// Wrap оборачивает нижележащую ошибку в доменную. code оставлен строкой для совместимости,
// Kind подбирается по registry, если известен.
func Wrap(err error, code, message string) *AppError {
	return &AppError{
		Code:    code,
		Kind:    ParseKind(code),
		Message: message,
		Err:     err,
	}
}

// Wrapf — форматированный вариант Wrap.
func Wrapf(err error, code, format string, args ...interface{}) *AppError {
	return Wrap(err, code, fmt.Sprintf(format, args...))
}

// WithDetail добавляет машиночитаемую деталь к ошибке. Для AppError возвращает
// поверхностную копию, чтобы оригинал не менялся.
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
